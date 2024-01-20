package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type bench struct {
	requests    uint
	concurrency uint
	timeout     uint

	host   string
	method string
	params url.Values
	data   map[string]any

	stats  stats
	client http.Client
}

type stats struct {
	LaunchTime time.Time
	Runtime    time.Duration

	RequestsPerSecond uint32
	RequestsTotal     uint32
	RequestsSuccess   uint32
	RequestsFail      uint32
	RequestsTimeout   uint32

	DelayMin time.Duration
	DelayAvg time.Duration
	DelayMax time.Duration
}

type task struct {
	url    string
	method string
	data   io.Reader
}

func NewBench() bench {
	return bench{}
}

func (b *bench) ParseArgs() error {
	numRequest := flag.Uint("n", 1000, "Number of requests")
	concurrency := flag.Uint("c", 1, "Concurrency")
	timeout := flag.Uint("t", 100, "Request timeout, ms")
	host := flag.String("h", "", "Target URL address")
	method := flag.String("m", "GET", "Request method")
	params := flag.String("p", "", "Request params")
	flag.Parse()

	b.requests = *numRequest
	b.concurrency = *concurrency
	b.timeout = *timeout

	switch *method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		b.method = *method
	default:
		return errors.New("unsupported HTTP method")
	}

	if u, err := url.ParseRequestURI(*host); err != nil {
		return errors.New("invalid URL")
	} else {
		b.host = u.String()
	}
	if p, err := url.ParseQuery(*params); err == nil {
		b.params = p
	}
	b.client = http.Client{
		Timeout: time.Millisecond * time.Duration(b.timeout),
	}
	return nil
}

func (b *bench) Run() {
	b.stats.LaunchTime = time.Now()
	numRequests := b.requests / b.concurrency
	var wg sync.WaitGroup
	task := task{
		url: fmt.Sprintf("%s?%s", b.host, b.params.Encode()),
	}

	if b.data != nil {
		data, _ := json.Marshal(b.data)
		task.data = bytes.NewBuffer(data)
	}

	for i := uint(0); i < b.concurrency; i++ {
		wg.Add(1)
		go func() {
			b.LaunchTask(numRequests, task)
			wg.Done()
		}()
	}
	wg.Wait()
}

func (b *bench) LaunchTask(numRequest uint, t task) {
	req, err := http.NewRequest(t.method, t.url, nil)

	if err != nil {
		return
	}
	for i := uint(0); i < numRequest; i++ {
		start := time.Now()
		atomic.AddUint32(&b.stats.RequestsTotal, 1)
		resp, err := b.client.Do(req)

		if err != nil {
			atomic.AddUint32(&b.stats.RequestsFail, 1)

			if err == http.ErrHandlerTimeout {
				atomic.AddUint32(&b.stats.RequestsTimeout, 1)
			}
		} else if resp.StatusCode == http.StatusOK {
			atomic.AddUint32(&b.stats.RequestsSuccess, 1)
		}
		delay := time.Since(start)

		if b.stats.DelayMin == 0 || delay < b.stats.DelayMin {
			b.stats.DelayMin = delay
		}
		if delay > b.stats.DelayMax {
			b.stats.DelayMax = delay
		}

	}
}

func (b *bench) PrintResult() {
	b.stats.Runtime = time.Since(b.stats.LaunchTime)
	rps := float64(b.stats.RequestsTotal) / b.stats.Runtime.Seconds()
	b.stats.DelayAvg = (b.stats.DelayMax - b.stats.DelayMin) / 2
	res := fmt.Sprintf(`
		Runtime: %s
		Concurrency: %d
		Requests per second: %2.f

		Total requests: %d
		Success requests: %d
		Fail requests: %d
		Timeout requests: %d

		Min delay: %s
		Avg delay: %s
		Max delay: %s
	`,
		b.stats.Runtime,
		b.concurrency,
		rps,
		b.stats.RequestsTotal,
		b.stats.RequestsSuccess,
		b.stats.RequestsFail,
		b.stats.RequestsTimeout,
		b.stats.DelayMin,
		b.stats.DelayAvg,
		b.stats.DelayMax,
	)
	fmt.Println(res)
}
