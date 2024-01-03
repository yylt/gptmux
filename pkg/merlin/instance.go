package merlin

import (
	"fmt"
	"time"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"k8s.io/klog/v2"
)

var (
	modelCost = map[string]int{
		"GPT 3":            1,
		"GPT 4":            30,
		"claude-instant-1": 2,
		"claude-2":         8,
		// t2i
		"Dreamshape v7": 10,
		"RPG v5":        10,
	}
	defaultImageModel = "Dreamshape v7"
	defaultChatModel  = "GPT 3"
)

type instance struct {
	idtoken     string // status
	accesstoken string // chat
	user        string
	password    string
	used        int
	limit       int
}

func (c *instance) DeepCopy() *instance {
	return &instance{
		accesstoken: c.accesstoken,
		idtoken:     c.idtoken,
		user:        c.user,
		password:    c.password,
		used:        c.used,
		limit:       c.limit,
	}
}

func instCompare(a, b interface{}) int {
	s1 := a.(*instance)
	s2 := b.(*instance)
	if s1.used > s2.used {
		return -1
	} else if s1.used == s2.used {
		return 0
	}
	return 1
}

type instCtrl struct {
	interval time.Duration

	ml *Merlin

	queue *priorityqueue.Queue
}

func NewInstControl(d time.Duration, ml *Merlin, user []*User) *instCtrl {
	queue := priorityqueue.NewWith(instCompare)
	for _, u := range user {
		queue.Enqueue(&instance{
			user:     u.User,
			password: u.Password,
		})
	}
	ic := &instCtrl{
		interval: d,
		ml:       ml,
		queue:    queue,
	}
	go ic.run()
	return ic
}

func (ic *instCtrl) allocate(m string) (*instance, error) {
	cost, ok := modelCost[m]
	if !ok {
		return nil, fmt.Errorf("not found model %v", m)
	}
	v, ok := ic.queue.Peek()
	if !ok {
		return nil, fmt.Errorf("not found instance")
	}
	inst := v.(*instance)
	if inst.limit-inst.used < cost {
		return nil, fmt.Errorf("no avaliable token to use")
	}
	return inst, nil
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
			if v.accesstoken != "" && ic.ml.status(v) == nil {
				continue
			}
			err := ic.ml.refresh(v)
			if err != nil {
				klog.Errorf("merlin user(%s/%s) login failed: %v", v.user, v.password, err)
			} else {
				klog.Infof("user(%s/%s) limit %d used %d.", v.user, v.password, v.limit, v.used)
			}
		}

		<-time.NewTicker(interval).C
	}
}
