package pkg

import (
	"bytes"
	"fmt"
	"net/url"
	"sync"
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

func Validurl(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("url is null")
	}
	return nil

}

func IsHttp20xCode(num int) bool {
	return num >= 200 && num <= 299
}
