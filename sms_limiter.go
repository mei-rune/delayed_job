package delayed_job

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var smsLimiter *SmsLimiter

type smsdata struct {
	TS    int32 `json:"ts"`
	Count int32 `json`
}

type SmsLimiter struct {
	mu         sync.Mutex
	filename   string
	dayLimit   int32
	weekLimit  int32
	monthLimit int32
	data       []smsdata
}

func (smsLimiter *SmsLimiter) canSendByDay(day int32) bool {
	found := -1
	for idx := range smsLimiter.data {
		if smsLimiter.data[idx].TS == day {
			found = idx
			break
		}
	}

	if found < 0 {
		return true
	}

	return smsLimiter.data[found].Count < smsLimiter.dayLimit
}

func (smsLimiter *SmsLimiter) countByRange(day, rangeValue int32) int32 {
	count := int32(0)
	for idx := range smsLimiter.data {
		if smsLimiter.data[idx].TS > (day - rangeValue) {
			count += smsLimiter.data[idx].Count
		}
	}
	return count
}

func (smsLimiter *SmsLimiter) CanSend() bool {
	smsLimiter.mu.Lock()
	defer smsLimiter.mu.Unlock()

	ts := time.Now()
	year := ts.Year()
	day := int32(year)*10000 + int32(ts.YearDay())

	if !smsLimiter.canSendByDay(day) {
		return false
	}

	if smsLimiter.weekLimit > 0 && smsLimiter.countByRange(day, 7) > smsLimiter.weekLimit {
		return false
	}

	if smsLimiter.monthLimit > 0 && smsLimiter.countByRange(day, 30) > smsLimiter.monthLimit {
		return false
	}
	return true
}

func (smsLimiter *SmsLimiter) Add(count int) {
	smsLimiter.mu.Lock()
	defer smsLimiter.mu.Unlock()

	ts := time.Now()
	year := ts.Year()
	day := int32(year)*10000 + int32(ts.YearDay())

	found := -1
	for idx := range smsLimiter.data {
		if smsLimiter.data[idx].TS == day {
			found = idx
			break
		}
	}

	if found < 0 {
		smsLimiter.data = append(smsLimiter.data, smsdata{TS: day})
		found = len(smsLimiter.data) - 1
	}

	smsLimiter.data[found].Count += int32(count)

	bs, err := json.Marshal(smsLimiter.data)
	if err == nil {
		ioutil.WriteFile(smsLimiter.filename, bs, 0666)
	}
}

func NewSMSLimiter(filename string, dayLimit, weekLimit, monthLimit int32) (*SmsLimiter, error) {
	limiter := &SmsLimiter{
		filename:   filename,
		dayLimit:   dayLimit,
		weekLimit:  weekLimit,
		monthLimit: monthLimit,
	}

	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		err = json.Unmarshal(bs, &limiter.data)
		if err != nil {
			log.Println("载入 sms limiter 失败", err)
		} else if len(limiter.data) > 365 {
			limiter.data = limiter.data[:365]
		}
	}
	return limiter, nil
}

func SetSMSLimiter(limiter *SmsLimiter) {
	smsLimiter = limiter
}
