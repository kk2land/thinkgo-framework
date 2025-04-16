package thinkgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type JMap map[string]interface{}

func JString(i interface{}) (string, bool) {
	if i != nil {
		switch d := i.(type) {
		case string:
			return d, true
		case int64:
			return strconv.FormatInt(d, 10), true
		case float64:
			return strconv.FormatFloat(d, 'f', -1, 64), true
		case bool:
			if d {
				return "true", true
			} else {
				return "false", true
			}
		case []interface{}:
			b, _ := json.Marshal(d)
			return string(b), true
		case map[string]interface{}:
			b, _ := json.Marshal(d)
			return string(b), true
		default:
			if b, err := json.Marshal(i); err != nil {
				return "", false
			} else {
				return string(b), true
			}
		}
	}
	return "", false
}

func JInt(i interface{}) (int64, bool) {
	if i != nil {
		switch d := i.(type) {
		case string:
			if j, err := strconv.Atoi(d); err != nil {
				return 0, false
			} else {
				return int64(j), true
			}
		case int64:
			return d, true
		case float64:
			return int64(d), true
		case bool:
			if d {
				return 1, true
			} else {
				return 0, true
			}
		}
	}
	return 0, false
}

func JFloat(i interface{}) (float64, bool) {
	if i != nil {
		switch d := i.(type) {
		case string:
			if j, err := strconv.ParseFloat(d, 64); err != nil {
				return 0.0, false
			} else {
				return j, true
			}
		case int64:
			return float64(d), true
		case float64:
			return d, true
		case bool:
			if d {
				return 1.0, true
			} else {
				return 0.0, true
			}
		}
	}
	return 0.0, false
}

func JBool(i interface{}) (bool, bool) {
	if i != nil {
		switch d := i.(type) {
		case string:
			return len(d) != 0, true
		case int64:
			return d != 0, true
		case float64:
			return d != 0, true
		case bool:
			return d, true
		case []interface{}:
			return len(d) != 0, true
		case map[string]interface{}:
			return len(d) != 0, true
		}
	}
	return false, false
}

func (m JMap) GetString(key string) (string, bool) {
	if i, ok := m[key]; !ok {
		return "", ok
	} else {
		return JString(i)
	}
}

func (m JMap) GetStringDefault(key string, dft string) string {
	if i, ok := m.GetString(key); ok {
		return i
	} else {
		return dft
	}
}

func (m JMap) GetInt(key string) (int64, bool) {
	if i, ok := m[key]; !ok {
		return 0, false
	} else {
		return JInt(i)
	}
}

func (m JMap) GetIntDefault(key string, dft int64) int64 {
	if i, ok := m.GetInt(key); ok {
		return i
	} else {
		return dft
	}
}

func (m JMap) GetFloat(key string) (float64, bool) {
	if i, ok := m[key]; !ok {
		return 0.0, ok
	} else {
		return JFloat(i)
	}
}

func (m JMap) GetFloatDefault(key string, dft float64) float64 {
	if i, ok := m.GetFloat(key); ok {
		return i
	} else {
		return dft
	}
}

func (m JMap) GetBoolDefault(key string, dft bool) bool {
	if i, ok := m[key]; ok {
		if b, ok := JBool(i); ok {
			return b
		}
	}
	return dft
}

func (m JMap) GetAny(key string) (interface{}, bool) {
	i, ok := m[key]
	return i, ok
}

func (m JMap) GetMap(key string) (JMap, bool) {
	if i, ok := m[key]; ok {
		if subm, ok := i.(map[string]interface{}); ok {
			return subm, true
		}
	}
	return nil, false
}

func (m JMap) GetArr(key string) ([]interface{}, bool) {
	if i, ok := m[key]; ok {
		if suba, ok := i.([]interface{}); ok {
			return suba, true
		}
	}
	return nil, false
}

func (m JMap) GetStruct(key string, v interface{}) error {
	if i, ok := m[key]; ok {
		b, err := json.Marshal(i)
		if err != nil {
			return fmt.Errorf("key(%s)值(%v)json_encode失败,err=%s", key, i, err)
		}
		if err = json.Unmarshal(b, &v); err != nil {
			return fmt.Errorf("key(%s)值(%s)json_decode失败,err=%s", key, string(b), err)
		}
		return nil
	}
	return fmt.Errorf("key不存在-%s", key)
}

func (m JMap) ToStruct(v interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("map(%v)json_encode失败,err=%s", m, err)
	}
	if err = json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("map(%s)json_decode失败,err=%s", string(b), err)
	}
	return nil
}

func (m *JMap) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	if b[0] == '"' {
		if len(b) < 2 {
			return nil
		} else if b[1] != '{' {
			return fmt.Errorf("json解析不是map字符串-%s", string(b))
		}
		b = b[1 : len(b)-1]
	} else if b[0] != '{' {
		return fmt.Errorf("json解析不是map/字符串-%s", string(b))
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*m = tmp
	return nil
}

func (m JMap) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}
