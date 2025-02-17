package merlin

import (
	"fmt"

	"k8s.io/klog/v2"
)

var (
	defaultImageModel = "Dreamshape v7"
	defaultChatModel  = "gpt-4o-mini"
)

type instance struct {
	accesstoken string // chat
	idtoken     string
	user        string
	password    string
	used        int
	limit       int
}

func (c *instance) DeepCopy() *instance {
	return &instance{
		accesstoken: c.accesstoken,
		user:        c.user,
		password:    c.password,
		used:        c.used,
		limit:       c.limit,
	}
}
func (c *instance) String() string {
	return fmt.Sprintf("user: %s(%d/%d)", c.user, c.used, c.limit)
}

func instCompare(a, b interface{}) int {
	s1 := a.(*instance)
	s2 := b.(*instance)
	if s1.used > s2.used {
		return 1
	} else if s1.used == s2.used {
		return 0
	}
	return -1
}

func NewInstance(m *Merlin, u *user) *instance {
	ins := &instance{
		user:     u.User,
		password: u.Password,
	}
	err := m.access(ins)
	if err != nil {
		klog.Errorf("access user %s, error: %v", ins, err)
		return nil
	}
	return ins
}
