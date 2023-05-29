package spider

import (
	"errors"
	"github.com/lizongying/go-crawler/pkg"
	"reflect"
	"sort"
)

func (s *BaseSpider) GetMiddlewares() (middlewares map[int]string) {
	middlewares = make(map[int]string)
	for k, v := range s.middlewares {
		middlewares[k] = reflect.TypeOf(v).Elem().String()
	}

	return
}

func (s *BaseSpider) ReplaceMiddlewares(middlewares map[int]pkg.Middleware) (err error) {
	middlewaresNameMap := make(map[string]struct{})
	middlewaresOrderMap := make(map[int]struct{})
	for k, v := range middlewares {
		name := reflect.TypeOf(v).Elem().String()
		if _, ok := middlewaresNameMap[name]; ok {
			err = errors.New("middleware name duplicate")
			s.Logger.Error(err)
			return
		}
		middlewaresNameMap[name] = struct{}{}
		if _, ok := middlewaresOrderMap[k]; ok {
			err = errors.New("middleware order duplicate")
			s.Logger.Error(err)
			return
		}
		middlewaresOrderMap[k] = struct{}{}
	}

	s.middlewares = middlewares

	return
}

func (s *BaseSpider) SetMiddleware(NewMiddleware func() pkg.Middleware, order int) pkg.Spider {
	middleware := NewMiddleware().FromCrawler(s)
	name := reflect.TypeOf(middleware).Elem().String()
	for k, v := range s.middlewares {
		if reflect.TypeOf(v).Elem().String() == name && k != order {
			delete(s.middlewares, k)
			break
		}
	}

	s.middlewares[order] = middleware

	return s
}

func (s *BaseSpider) DelMiddleware(name string) {
	for k, v := range s.middlewares {
		if reflect.TypeOf(v).Elem().String() == name {
			delete(s.middlewares, k)
			break
		}
	}

	return
}

func (s *BaseSpider) CleanMiddlewares() {
	s.middlewares = make(map[int]pkg.Middleware)

	return
}

func (s *BaseSpider) SortedMiddlewares() (o []pkg.Middleware) {
	keys := make([]int, 0)
	for k := range s.middlewares {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, key := range keys {
		o = append(o, s.middlewares[key])
	}

	return
}
