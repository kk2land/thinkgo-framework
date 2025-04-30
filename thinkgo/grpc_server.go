package thinkgo

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const grpcMetaDataAuthentication = "tk-authorization"

var grpcServerOnce sync.Once
var grpcServer *grpc.Server = nil

type grpcStatusErr struct {
	code codes.Code
	msg  string
}

func (err *grpcStatusErr) GRPCStatus() *status.Status {
	return status.New(err.code, err.msg)
}

func (err *grpcStatusErr) Error() string {
	return err.msg
}

func grpcGetValue(m map[string][]string, k string) (string, bool) {
	if val, ok := m[k]; ok && len(val) > 0 {
		return val[0], true
	} else {
		return "", false
	}
}

func GrpcLogger(ctx context.Context) FieldLogger {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Logger
	}
	var reqid string
	if reqid, ok = grpcGetValue(md, loggerFieldId); !ok {
		return Logger
	}
	return Logger.With(loggerFieldId, reqid)
}

func grpcAuthentication(ctx context.Context, fullMethod string) error {
	logger := GrpcLogger(ctx)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Warnf("metadata.FromIncomingContext fail")
		return status.Errorf(codes.Unauthenticated, "Authentication error")
	}

	var auth string
	if auth, ok = grpcGetValue(md, grpcMetaDataAuthentication); !ok {
		logger.Warnf("gRPC校验,metadata[%s]为空", grpcMetaDataAuthentication)
		return status.Errorf(codes.Unauthenticated, "Authentication empty")
	}
	var host string
	if host, ok = grpcGetValue(md, ":authority"); !ok {
		logger.Warnf("gRPC校验,metadata[:authority]为空")
		return status.Errorf(codes.Unauthenticated, "Authentication empty authority")
	}
	host = strings.SplitN(host, ":", 2)[0]

	q, err := url.ParseQuery(auth)
	if err != nil {
		logger.Warnf("gRPC校验,auth字符串格式不对-%s", auth)
		return status.Errorf(codes.Unauthenticated, "Authentication invalid")
	}
	appid, ok1 := grpcGetValue(q, "appid")
	nonce, ok2 := grpcGetValue(q, "nonce")
	time1, ok3 := grpcGetValue(q, "time")
	sign, ok4 := grpcGetValue(q, "sign")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		logger.Warnf("gRPC校验,auth字符串格式不对,appid/nonce/time/sign存在空-%s", auth)
		return status.Errorf(codes.Unauthenticated, "Authentication invalid")
	}
	path := filepath.Dir(fullMethod)
	//"appid={$appid}nonce={$nonce}time={$time}url={$host}{$path}{$key}";
	buf := strings.Builder{}
	buf.WriteString("appid=")
	buf.WriteString(appid)
	buf.WriteString("nonce=")
	buf.WriteString(nonce)
	buf.WriteString("time=")
	buf.WriteString(time1)
	buf.WriteString("url=")
	buf.WriteString(host)
	buf.WriteString(path)
	buf.WriteString(Config.Grpc.AuthenticationKey)
	if Md5(buf.String()) != sign {
		logger.Warnf("gRPC校验,校验失败,str=%s,sign=%s", buf.String(), sign)
		return status.Errorf(codes.Unauthenticated, "Authentication fail")
	}
	return nil
}

func grpcLoggerError(ctx context.Context, err interface{}) error {
	stack := Stack(4, 5)
	logger := GrpcLogger(ctx)
	logger.Errorf("[GrpcHandleError] err=%s\n%s", err, stack)
	var msg string
	var code = codes.Unknown
	if !AppDebug {
		if apiErr, ok := err.(*ApiError); ok {
			msg = apiErr.apiMessage
			if apiErr.code != 0 {
				code = codes.Code(apiErr.code)
			}
		} else {
			msg = "internal error"
		}
	} else {
		msg = fmt.Sprintf("%s\n%s", err, stack)
	}
	return &grpcStatusErr{code: code, msg: msg}
}

func grpcAuthUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (obj interface{}, err error) {
	if err = grpcAuthentication(ctx, info.FullMethod); err != nil {
		return
	}
	defer func() {
		if err1 := recover(); err1 != nil {
			err = grpcLoggerError(ctx, err1)
		}
	}()
	return handler(ctx, req)
}

func grpcAuthStreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	if err = grpcAuthentication(ss.Context(), info.FullMethod); err != nil {
		return
	}
	defer func() {
		if err1 := recover(); err1 != nil {
			err = grpcLoggerError(ss.Context(), err1)
		}
	}()
	return handler(srv, ss)
}

func grpcErrUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (obj interface{}, err error) {
	defer func() {
		if err1 := recover(); err1 != nil {
			err = grpcLoggerError(ctx, err1)
		}
	}()
	return handler(ctx, req)
}

func grpcErrStreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if err1 := recover(); err1 != nil {
			err = grpcLoggerError(ss.Context(), err1)
		}
	}()
	return handler(srv, ss)
}

func GrpcEnable() bool {
	switch CommandName {
	case CommandNameHttp:
		return Config.Http.WithGrpc
	case CommandNameGrpc:
		return true
	}
	return false
}

// GrpcServer 获取GrpcServer
func GrpcServer() *grpc.Server {
	grpcServerOnce.Do(func() {
		//todo 读取配置初始化server
		var opts []grpc.ServerOption
		if Config.Grpc.AuthenticationEnable {
			if len(Config.Grpc.AuthenticationKey) == 0 {
				panic(fmt.Errorf("config[grpc][authenticationEnable]开启时，config[grpc][authenticationKey]不能为空"))
			}
			opts = append(opts,
				grpc.UnaryInterceptor(grpcAuthUnaryServerInterceptor),
				grpc.StreamInterceptor(grpcAuthStreamServerInterceptor))
		} else {
			opts = append(opts,
				grpc.UnaryInterceptor(grpcErrUnaryServerInterceptor),
				grpc.StreamInterceptor(grpcErrStreamServerInterceptor))
		}
		grpcServer = grpc.NewServer(opts...)
	})
	return grpcServer
}

func grpcHandleError(logger FieldLogger, w http.ResponseWriter, r *http.Request, err interface{}) {
	stack := Stack(4, 5)
	brokenPipe := ErrIsBrokenPipe(err)
	var msg = "internal error"
	logger.Errorf("[GrpcHandleError] err=%s\n%s", err, stack)
	if AppDebug {
		msg = fmt.Sprintf("%s\n%s", err, stack)
	}
	if !brokenPipe {
		var ret = "0"
		if !AppDebug {
			if apiErr, ok := err.(*ApiError); ok {
				ret = fmt.Sprintf("%d", apiErr.code)
				msg = apiErr.apiMessage
			}
		}
		w.Header().Set("Content-Type", "application/grpc+proto")
		w.Header().Set("grpc-status", ret)
		w.Header().Set("grpc-message", url.QueryEscape(msg))
		w.WriteHeader(200)
	}
}

func grpcHttpHandler(elseHandler http.Handler) http.Handler {
	var server = GrpcServer()
	f := func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			reqId := httpIncrReqId()
			logger := Logger.With(
				loggerFieldId, reqId,
				"ip", ClientIp(r),
				"path", r.URL.Path,
			)
			start := time.Now()
			defer func() {
				if err := recover(); err != nil {
					grpcHandleError(logger, w, r, err)
				}
				if AppDebug {
					code := w.Header().Get("grpc-status")
					if len(code) == 0 {
						code = "200"
					}
					logger.Debugf("[GRPC] %s | %s", code, time.Now().Sub(start))
				}
			}()
			r.Header.Set(loggerFieldId, reqId)
			server.ServeHTTP(w, r)
		} else {
			elseHandler.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(f)
}
