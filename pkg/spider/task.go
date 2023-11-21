package spider

import (
	"errors"
	"fmt"
	"github.com/lizongying/go-crawler/pkg"
	crawlerContext "github.com/lizongying/go-crawler/pkg/context"
	kafka2 "github.com/lizongying/go-crawler/pkg/scheduler/kafka"
	"github.com/lizongying/go-crawler/pkg/scheduler/memory"
	redis2 "github.com/lizongying/go-crawler/pkg/scheduler/redis"
	"github.com/lizongying/go-crawler/pkg/stats"
	"reflect"
	"time"
)

type Task struct {
	context        pkg.Context
	request        *pkg.State
	item           *pkg.State
	requestAndItem *pkg.MultiState
	crawler        pkg.Crawler
	spider         pkg.Spider
	logger         pkg.Logger
	job            *Job
	pkg.Stats
	pkg.Scheduler
}

func (t *Task) GetContext() pkg.Context {
	return t.context
}
func (t *Task) WithContext(ctx pkg.Context) *Task {
	t.context = ctx
	return t
}
func (t *Task) GetScheduler() pkg.Scheduler {
	return t.Scheduler
}
func (t *Task) WithScheduler(scheduler pkg.Scheduler) pkg.Task {
	t.Scheduler = scheduler
	return t
}
func (t *Task) start(ctx pkg.Context) (id string, err error) {
	id = t.crawler.NextId()
	if t.GetContext() == nil {
		t.WithContext(new(crawlerContext.Context).
			WithCrawler(ctx.GetCrawler()).
			WithSpider(ctx.GetSpider()).
			WithJob(ctx.GetJob()).
			WithTask(new(crawlerContext.Task).
				WithTask(t).
				WithContext(ctx.GetJob().GetContext()).
				WithId(id).
				WithJobSubId(ctx.GetJob().GetSubId()).
				WithStatus(pkg.TaskStatusPending).
				WithStartTime(time.Now()).
				WithStats(new(stats.MediaStats))))
		t.crawler.GetSignal().TaskChanged(t.context)

		t.logger.Info(t.spider.Name(), id, "task started")
	}

	if err = t.StartScheduler(t.context); err != nil {
		t.logger.Error(err)
		return
	}

	go func() {
		select {
		case <-t.context.GetTask().GetContext().Done():
			if t.context.GetTask().GetStatus() < pkg.TaskStatusSuccess {
				t.stop(t.context.GetTask().GetContext().Err())
			}
			return
		}
	}()

	go func() {

		defer func() {
			//if r := recover(); r != nil {
			//	s.logger.Error(r)
			//}
		}()

		params := []reflect.Value{
			reflect.ValueOf(t.context),
			reflect.ValueOf(t.context.GetJob().GetArgs()),
		}
		caller := reflect.ValueOf(t.spider).MethodByName(t.context.GetJob().GetFunc())
		if !caller.IsValid() {
			err = errors.New(fmt.Sprintf("schedule func is invalid: %s", t.context.GetJob().GetFunc()))
			t.logger.Error(err)
			return
		}

		res := caller.Call(params)
		if len(res) != 1 {
			err = errors.New(fmt.Sprintf("%s has too many return values", t.context.GetJob().GetFunc()))
			t.logger.Error(err)
			return
		}

		if res[0].Type().Name() != "error" {
			err = errors.New(fmt.Sprintf("%s should return an error", t.context.GetJob().GetFunc()))
			t.logger.Error(err)
			return
		}

		if !res[0].IsNil() {
			err = res[0].Interface().(error)
			t.logger.Error(err)
			return
		}
	}()

	return
}
func (t *Task) stop(err error) {
	_ = t.StopScheduler(t.context)

	if err != nil {
		t.context.GetTask().WithStopReason(err.Error())
		t.context.GetTask().WithStatus(pkg.TaskStatusFailure)
	} else {
		t.context.GetTask().WithStatus(pkg.TaskStatusSuccess)
	}
	t.crawler.GetSignal().TaskChanged(t.context)
	t.job.TaskStopped(t.context, err)
	return
}
func (t *Task) RequestPending(_ pkg.Context, _ error) {
	t.request.BeReady()
}
func (t *Task) RequestRunning(_ pkg.Context, err error) {
	if err != nil {
		return
	}
	t.request.In()
}
func (t *Task) RequestStopped(_ pkg.Context, _ error) {
	t.request.Out()
}
func (t *Task) ItemPending(_ pkg.Context, _ error) {
	t.item.BeReady()
}
func (t *Task) ItemRunning(ctx pkg.Context, err error) {
	if err != nil {
		return
	}
	t.item.In()
}
func (t *Task) ItemStopped(_ pkg.Context, _ error) {
	//item := ctx.GetItem()
	//item.WithContext()
	t.item.Out()
}
func (t *Task) WithJob(job *Job) *Task {
	t.job = job
	return t
}
func (t *Task) FromSpider(spider pkg.Spider) *Task {
	*t = Task{
		crawler: spider.GetCrawler(),
		spider:  spider,
		logger:  spider.GetLogger(),
		request: pkg.NewState(),
		item:    pkg.NewState(),
	}

	t.requestAndItem = pkg.NewMultiState(t.request, t.item)

	t.requestAndItem.RegisterIsReadyAndIsZero(func() {
		t.stop(nil)
	})

	config := spider.GetConfig()

	switch config.GetScheduler() {
	case pkg.SchedulerMemory:
		t.WithScheduler(new(memory.Scheduler).FromSpider(spider))
	case pkg.SchedulerRedis:
		t.WithScheduler(new(redis2.Scheduler).FromSpider(spider))
	case pkg.SchedulerKafka:
		t.WithScheduler(new(kafka2.Scheduler).FromSpider(spider))
	default:
	}

	return t
}
