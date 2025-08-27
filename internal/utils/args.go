package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"tgautodown/internal/logs"
)

type xmv struct {
	desc string
	def  any
	miss string
}

var xmKVs map[string]xmv

func init() {
	xmKVs = make(map[string]xmv)
}

func XmUsage() {
	xmDefaultUsage(os.Args[0])
}
func xmDefaultUsage(self string) {
	fmt.Printf("usage: %s options\n", self)
	// fmt.Printf("  -logpath <path-to-logger>\n")
	// fmt.Printf("  -loglev 1    # -1 trace, 0 debug, 1 info, 2 warn, 3 error, 5 fatal, 5 panic, 6 nolevel\n")
	// fmt.Printf("  -logsize 300 # unit: MB\n")
	// fmt.Printf("  -logcnt 1    # max log count\n")

	for k, v := range xmKVs {
		if rv, ok := v.def.(bool); ok {
			if v.desc == "" {
				fmt.Printf("  -%s%s    ## default %t\n", k, v.miss, rv)
			} else {
				fmt.Printf("  -%s%s    ## %v, default %t\n", k, v.miss, v.desc, rv)
			}
		} else {
			if v.desc == "" {
				fmt.Printf("  -%s%s %v\n", k, v.miss, v.def)
			} else {
				fmt.Printf("  -%s%s %v  ## %v\n", k, v.miss, v.def, v.desc)
			}
		}
	}
	os.Exit(2)
}

func xmArgVal(name string) (string, bool) {
	argv := os.Args[1:]
	argc := len(argv)
	for i, k := range argv {
		kn := strings.TrimLeft(k, "-")
		ks := strings.SplitAfterN(kn, "=", 2)
		if len(ks) == 1 { // 不包含 =
			// log.Printf("name=%s, i=%d, k=%s, kn=%s, ks==1\n", name, i, k, kn)
			if kn == k { // not valid key
				continue
			}

			if kn == name {
				if i+1 < argc {
					v := argv[i+1]
					vn := strings.TrimLeft(v, "-") // 检查 value 是否以 - 开头
					if vn == v {                   // 不以 - 开头，是正常的value
						return v, true
					} else { // 以 - 开头，说明是下一个key了
						return "", true
					}
				} else {
					return "", true
				}
			}
		} else { // 包含=，说明value存在于 k 中
			kn = strings.TrimRight(ks[0], "=")
			// log.Printf("name=%s, i=%d, k=%s, kn=%s, ks=%v\n", name, i, k, kn, ks)
			if kn == k { // not valid key
				continue
			}
			if kn == name { // 不以 - 开头，是正常的value
				return ks[1], true
			}
		}
	}
	return "", false
}

func XmArgValString(name string, desc, defVal string) string {
	xmKVs[name] = xmv{def: defVal, desc: desc}
	if v, ok := xmArgVal(name); ok {
		return v
	}
	return defVal
}
func XmArgValInt64(name string, desc string, defVal int64) int64 {
	xmKVs[name] = xmv{def: defVal, desc: desc}
	if v, ok := xmArgVal(name); ok {
		if iv, err := strconv.ParseInt(v, 10, 64); err != nil {
			return defVal
		} else {
			return iv
		}
	}
	return defVal
}
func XmArgValInt(name string, desc string, defVal int) int {
	return int(XmArgValInt64(name, desc, int64(defVal)))
}
func XmArgValBool(name string, desc string) bool {
	xmKVs[name] = xmv{def: false, desc: desc}
	_, ok := xmArgVal(name)
	return ok
}

func XmArgValStrings(name string, desc, defVal string) []string {
	vals := XmArgValString(name, desc, defVal)
	// return strings.Split(vals, ",")
	sVals := strings.Split(vals, ",")

	res := []string{}
	for _, v := range sVals {
		res = append(res, strings.TrimSpace(v))
	}
	return res
}

func XmArgValInt64s(name string, desc string, defVal int64) []int64 {
	vals := XmArgValString(name, desc, strconv.FormatInt(defVal, 10))
	iVals := strings.Split(vals, ",")

	res := []int64{}
	for _, v := range iVals {
		if iv, err := strconv.ParseInt(v, 10, 64); err == nil {
			res = append(res, iv)
		}
	}
	return res
}
func XmArgValInts(name string, desc string, defVal int) []int {
	vals := XmArgValString(name, desc, strconv.FormatInt(int64(defVal), 10))
	iVals := strings.Split(vals, ",")

	res := []int{}
	for _, v := range iVals {
		if iv, err := strconv.ParseInt(v, 10, 64); err == nil {
			res = append(res, int(iv))
		}
	}
	return res
}

// func XmArgValsBool(names ...string) bool {
// 	return slices.ContainsFunc(names, XmArgValBool)
// }

// func XmCheckUsage(usage func(argv0 string), args ...string) {
// 	if XmArgValsBool(args...) {
// 		fmt.Printf("usage: %s options\n", os.Args[0])
// 		fmt.Printf("  -logpath <path-to-logger>\n")
// 		fmt.Printf("  -loglev 1    # -1 trace, 0 debug, 1 info, 2 warn, 3 error, 5 fatal, 5 panic, 6 nolevel\n")
// 		fmt.Printf("  -logsize 300 # unit: MB\n")
// 		fmt.Printf("  -logcnt 1    # max log count\n")
// 		usage(os.Args[0])
// 	}
// }

func XmUsageIfHasKeys(keys ...string) {
	for _, key := range keys {
		if _, ok := xmArgVal(key); ok {
			xmDefaultUsage(os.Args[0])
			return
		}
	}
}

func XmUsageIfHasNoKeys(keys ...string) {
	for _, key := range keys {
		if _, ok := xmArgVal(key); !ok {
			xmKVs[key] = xmv{def: false, miss: "[*miss*]"}
			xmDefaultUsage(os.Args[0])
			return
		}
	}
}

func XmLogsInit(path string, lev, size, cnt int) {
	logPath := XmArgValString("logpath", "", path)
	logLev := XmArgValInt("loglev", "-1 trace, 0 debug, 1 info, 2 warn, 3 error, 5 fatal, 5 panic, 6 nolevel", lev)
	logSize := XmArgValInt("logsize", "MB", size)
	logCnt := XmArgValInt("logcnt", "", cnt)

	logs.LogsInit(logPath, logLev, logSize<<20, logCnt)
}
