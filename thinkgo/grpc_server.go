package thinkgo

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type GrpcBeforeCreate func() []grpc.ServerOption

type GrpcAfterCreate func(server *grpc.Server)

type GrpcErrorHandler func(ctx context.Context, r interface{}) error

type GrpcErrorStatus struct {
	code    codes.Code
	message string
}

func (err *GrpcErrorStatus) GRPCStatus() *status.Status {
	return status.New(err.code, err.message)
}

func (err *GrpcErrorStatus) Error() string {
	return err.message
}

var (
	grpcBeforeCreateRef atomic.Value
	grpcAfterCreateRef  atomic.Value
	grpcErrorHandlerRef atomic.Value
)

func grpcHandleError(ctx context.Context, r interface{}) error {
	stack := Stack(4, 5)
	if ErrIsBrokenPipe(r) {
		return nil
	}
	GrpcLogger(ctx).Errorf("[GrpcHandleError] err=%v\n%s", r, stack)

	if errorHandler := grpcErrorHandlerRef.Load(); errorHandler != nil {
		return errorHandler.(GrpcErrorHandler)(ctx, r)
	} else {
		var code = codes.Unknown
		var message string
		if !AppDebug {
			message = "internal error"
		} else {
			message = fmt.Sprintf("%v\n%s", r, stack)
		}
		return &GrpcErrorStatus{code: code, message: message}
	}
}

func grpcErrUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (obj interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = grpcHandleError(ctx, r)
		}
	}()
	return handler(ctx, req)
}

func grpcErrStreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = grpcHandleError(ss.Context(), r)
		}
	}()
	return handler(srv, ss)
}

func GrpcOnBeforeCreate(f GrpcBeforeCreate) {
	grpcBeforeCreateRef.Store(f)
}

func GrpcOnAfterCreate(f GrpcAfterCreate) {
	grpcAfterCreateRef.Store(f)
}

func GrpcCreateServer() *grpc.Server {
	var opts []grpc.ServerOption
	opts = append(opts,
		grpc.UnaryInterceptor(grpcErrUnaryServerInterceptor),
		grpc.StreamInterceptor(grpcErrStreamServerInterceptor))
	if f := grpcBeforeCreateRef.Load(); f != nil {
		opts = append(opts, f.(GrpcBeforeCreate)()...)
	}
	server := grpc.NewServer(opts...)
	if f := grpcAfterCreateRef.Load(); f != nil {
		f.(GrpcAfterCreate)(server)
	}
	return server
}

// GrpcLogger 从请求context.Context中获取日志对象
func GrpcLogger(ctx context.Context) FieldLogger {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Logger
	}
	var reqID string
	if val, ok := md[loggerFieldId]; !ok || len(val) == 0 {
		return Logger
	} else {
		reqID = val[0]
	}
	return Logger.With(loggerFieldId, reqID)
}

func grpcHttpHandler(elseHandler http.Handler) http.Handler {
	server := GrpcCreateServer()
	f := func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			reqID := httpRequestId()
			logger := Logger.With(loggerFieldId, reqID, "ip", ClientIp(r), "path", r.URL.Path)
			start := time.Now()
			defer func() {
				if AppDebug {
					code := w.Header().Get("grpc-status")
					if len(code) == 0 {
						code = "200"
					}
					logger.Debugf("[GRPC] %s | %s", code, time.Now().Sub(start))
				}
			}()
			r.Header.Set(loggerFieldId, reqID)
			server.ServeHTTP(w, r)
		} else {
			elseHandler.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(f)
}
