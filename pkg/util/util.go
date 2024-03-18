package util

import (
	"bytes"
	"fmt"
	"net/url"
	"sync"
	"unicode"
	"unsafe"
)

var (
	bufpool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

func GetBuf() *bytes.Buffer {
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutBuf(b *bytes.Buffer) {
	bufpool.Put(b)
}

func ParseUrl(u string) (*url.URL, error) {
	appurl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	err = Validurl(appurl)
	if err != nil {
		return nil, err
	}
	return appurl, nil
}

func Str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	b := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&b))
}

func Bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func Validurl(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("url is null")
	}
	return nil

}

func IsHttp20xCode(num int) bool {
	return num >= 200 && num <= 299
}

func HasChineseChar(str string) bool {
	for _, r := range str {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func IsNewline(r rune) bool {

	return r == '\r' || r == '\n'
}
