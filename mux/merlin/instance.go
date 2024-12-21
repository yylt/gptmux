package merlin

import (
	"fmt"
	"time"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"k8s.io/klog/v2"
)

var (
	defaultImageModel = "Dreamshape v7"
	defaultChatModel  = "claude-3-haiku"
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
	return fmt.Sprintf("user: %s, used: %d, limit: %d", c.user, c.used, c.limit)
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

type instCtrl struct {
	interval time.Duration

	ml *Merlin

	queue *priorityqueue.Queue
}

func NewInstControl(d time.Duration, ml *Merlin, user []*user) *instCtrl {
	queue := priorityqueue.NewWith(instCompare)
	for _, u := range user {
		in := &instance{
			user:     u.User,
			password: u.Password,
		}
		err := ml.refresh(in)
		if err != nil {
			klog.Warning(err, fmt.Sprintf(", not valid user: %s", in.user))
			continue
		}
		queue.Enqueue(in)
	}
	ic := &instCtrl{
		interval: d,
		ml:       ml,
		queue:    queue,
	}
	if queue.Size() != 0 {
		go ic.run()
	}
	return ic
}

func (ic *instCtrl) Size() int {
	return ic.queue.Size()
}

func (ic *instCtrl) Eequeue(m *instance) {
	ic.queue.Enqueue(m)
}

func (ic *instCtrl) Dequeue() (*instance, error) {

	v, ok := ic.queue.Dequeue()
	if !ok {
		return nil, fmt.Errorf("no instance")
	}
	return v.(*instance), nil
}

func (ic *instCtrl) run() {
	var interval = ic.interval
	for {
		iter := ic.queue.Iterator()
		for iter.Next() {
			v, ok := iter.Value().(*instance)
			if !ok {
				continue
			}
			if v.accesstoken != "" && ic.ml.usage(v) == nil {
				klog.Infof("user(%s/%s) limit %d used %d.", v.user, v.password, v.limit, v.used)
				continue
			}
			err := ic.ml.refresh(v)
			if err != nil {
				klog.Errorf("refresh failed: %v", err)
			}
		}

		<-time.NewTimer(interval).C
	}
}
