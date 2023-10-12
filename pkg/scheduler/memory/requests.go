package memory

import (
	"context"
	"errors"
	"github.com/lizongying/go-crawler/pkg"
	"github.com/lizongying/go-crawler/pkg/request"
	"golang.org/x/time/rate"
	"reflect"
	"runtime"
	"time"
)

func (s *Scheduler) Request(ctx pkg.Context, request pkg.Request) (response pkg.Response, err error) {
	defer func() {
		s.Spider().StateRequest().Set()
	}()

	if request == nil {
		err = errors.New("nil request")
		return
	}

	s.logger.Debugf("request: %+v", request)

	response, err = s.Download(ctx, request)
	if err != nil {
		if errors.Is(err, pkg.ErrIgnoreRequest) {
			s.logger.Info(err)
			err = nil
			return
		}

		s.HandleError(ctx, response, err, request.ErrBack())
		return
	}

	s.logger.Debugf("request %+v", request)
	return
}

func (s *Scheduler) handleRequest(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	slot := "*"
	value, _ := s.RequestSlotLoad(slot)
	requestSlot := value.(*rate.Limiter)

	for requestWithContext := range s.requestWithContextChan {
		slot = requestWithContext.Slot()
		if slot == "" {
			slot = "*"
		}
		slotValue, ok := s.RequestSlotLoad(slot)
		if !ok {
			concurrency := uint8(1)
			if requestWithContext.Concurrency() != nil {
				concurrency = *requestWithContext.Concurrency()
			}
			if concurrency < 1 {
				concurrency = 1
			}
			requestSlot = rate.NewLimiter(rate.Every(requestWithContext.Interval()/time.Duration(concurrency)), int(concurrency))
			s.RequestSlotStore(slot, requestSlot)
		}

		requestSlot = slotValue.(*rate.Limiter)

		err := requestSlot.Wait(ctx)
		if err != nil {
			s.logger.Error(err, time.Now(), ctx)
		}
		go func(requestWithContext pkg.RequestWithContext) {
			c := requestWithContext.Global()
			response, e := s.Request(c, requestWithContext.GetRequest())
			if e != nil {
				s.Spider().StateRequest().Out()
				return
			}

			go func(ctx pkg.Context, response pkg.Response) {
				defer func() {
					if r := recover(); r != nil {
						buf := make([]byte, 1<<16)
						runtime.Stack(buf, true)
						err = errors.New(string(buf))
						//s.logger.Error(err)
						s.HandleError(ctx, response, err, requestWithContext.ErrBack())
					}
				}()

				s.Spider().StateMethod().In()
				if err = s.Spider().CallBack(requestWithContext.CallBack())(ctx, response); err != nil {
					s.logger.Error(err)
					s.HandleError(ctx, response, err, requestWithContext.ErrBack())
				}
				s.Spider().StateMethod().Out()
				s.Spider().StateRequest().Out()
			}(c, response)
		}(requestWithContext)
	}

	return
}

func (s *Scheduler) YieldRequest(ctx pkg.Context, req pkg.Request) (err error) {
	defer func() {
		s.Spider().StateRequest().Set()
	}()

	if len(s.requestWithContextChan) >= defaultRequestMax {
		err = errors.New("exceeded the maximum number of requests")
		s.logger.Error(err)
		return
	}

	meta := ctx.Meta()

	// add referrer to request
	if meta.Referrer != nil {
		req.SetReferrer(meta.Referrer.String())
	}

	// add cookies to request
	if len(meta.Cookies) > 0 {
		for _, cookie := range meta.Cookies {
			req.AddCookie(cookie)
		}
	}

	s.Spider().StateRequest().In()

	s.requestWithContextChan <- request.WithContext{
		Context: ctx,
		Request: req,
	}

	return
}

func (s *Scheduler) YieldExtra(extra any) (err error) {
	defer func() {
		s.Spider().StateRequest().In()
		s.Spider().StateRequest().Set()
	}()

	extraValue := reflect.ValueOf(extra)
	if extraValue.Kind() != reflect.Ptr || extraValue.IsNil() {
		err = errors.New("extra must be a non-null pointer")
		return
	}

	name := extraValue.Elem().Type().Name()
	extraChan, ok := s.extraChanMap.LoadOrStore(name, func() chan any {
		extraChan := make(chan any, defaultRequestMax)
		extraChan <- extra
		return extraChan
	}())
	if ok {
		extraChan.(chan any) <- extra
	}

	return
}

func (s *Scheduler) GetExtra(extra any) (err error) {
	defer func() {
		s.Spider().StateRequest().Out()
	}()

	extraValue := reflect.ValueOf(extra)
	if extraValue.Kind() != reflect.Ptr || extraValue.IsNil() {
		err = errors.New("extra must be a non-null pointer")
		return
	}

	name := extraValue.Elem().Type().Name()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.CloseReasonQueueTimeout())*time.Second)
	defer cancel()

	resultChan := make(chan struct{})
	go func() {
		extraChan, ok := s.extraChanMap.Load(name)
		if ok {
			extra = <-extraChan.(chan any)
			resultChan <- struct{}{}
		}
	}()

	select {
	case <-resultChan:
		return
	case <-ctx.Done():
		close(resultChan)
		err = pkg.ErrQueueTimeout
		return
	}
}
