package main

import (
	"fmt"
	"github.com/lizongying/go-crawler/pkg"
	"github.com/lizongying/go-crawler/pkg/app"
	"github.com/lizongying/go-crawler/pkg/mockServers"
	"github.com/lizongying/go-crawler/pkg/request"
)

type Spider struct {
	pkg.Spider
	logger pkg.Logger
}

func (s *Spider) ParseDecode(_ pkg.Context, response pkg.Response) (err error) {
	s.logger.Info("header", response.Headers())
	s.logger.Info("body", response.BodyStr())
	return
}

// TestGbk go run cmd/testDecodeSpider/*.go -c dev.yml -n test-decode -f TestGbk -m once
func (s *Spider) TestGbk(ctx pkg.Context, _ string) (err error) {
	s.AddMockServerRoutes(mockServers.NewRouteGbk(s.logger))

	err = s.YieldRequest(ctx, request.NewRequest().
		SetUrl(fmt.Sprintf("%s%s", s.GetHost(), mockServers.UrlGbk)).
		SetCallBack(s.ParseDecode))
	if err != nil {
		s.logger.Error(err)
		return
	}

	return
}

// TestGb2312 go run cmd/testDecodeSpider/*.go -c dev.yml -n test-decode -f TestGb2312 -m once
func (s *Spider) TestGb2312(ctx pkg.Context, _ string) (err error) {
	s.AddMockServerRoutes(mockServers.NewRouteGb2312(s.logger))

	err = s.YieldRequest(ctx, request.NewRequest().
		SetUrl(fmt.Sprintf("%s%s", s.GetHost(), mockServers.UrlGb2312)).
		SetCallBack(s.ParseDecode))
	if err != nil {
		s.logger.Error(err)
		return
	}

	return
}

// TestGb18030 go run cmd/testDecodeSpider/*.go -c dev.yml -n test-decode -f TestGb18030 -m once
func (s *Spider) TestGb18030(ctx pkg.Context, _ string) (err error) {
	s.AddMockServerRoutes(mockServers.NewRouteGb18030(s.logger))

	err = s.YieldRequest(ctx, request.NewRequest().
		SetUrl(fmt.Sprintf("%s%s", s.GetHost(), mockServers.UrlGb18030)).
		SetCallBack(s.ParseDecode))
	if err != nil {
		s.logger.Error(err)
		return
	}

	return
}

// TestBig5 go run cmd/testDecodeSpider/*.go -c dev.yml -n test-decode -f TestBig5 -m once
func (s *Spider) TestBig5(ctx pkg.Context, _ string) (err error) {
	s.AddMockServerRoutes(mockServers.NewRouteBig5(s.logger))

	err = s.YieldRequest(ctx, request.NewRequest().
		SetUrl(fmt.Sprintf("%s%s", s.GetHost(), mockServers.UrlBig5)).
		SetCallBack(s.ParseDecode))
	if err != nil {
		s.logger.Error(err)
		return
	}

	return
}

func NewSpider(baseSpider pkg.Spider) (spider pkg.Spider, err error) {
	spider = &Spider{
		Spider: baseSpider,
		logger: baseSpider.GetLogger(),
	}
	spider.WithOptions(
		pkg.WithName("test-decode"),
		pkg.WithHost("https://localhost:8081"),
	)
	return
}

func main() {
	app.NewApp(NewSpider).Run()
}
