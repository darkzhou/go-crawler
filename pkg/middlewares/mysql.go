package middlewares

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/lizongying/go-crawler/pkg"
	"reflect"
	"strings"
	"time"
)

type MysqlMiddleware struct {
	pkg.UnimplementedMiddleware
	logger pkg.Logger

	mysql   *sql.DB
	timeout time.Duration
	spider  pkg.Spider
	info    *pkg.SpiderInfo
	stats   pkg.Stats
}

func (m *MysqlMiddleware) SpiderStart(_ context.Context, spider pkg.Spider) (err error) {
	m.spider = spider
	m.info = spider.GetInfo()
	m.stats = spider.GetStats()
	return
}

func (m *MysqlMiddleware) ProcessItem(c *pkg.Context) (err error) {
	item, ok := c.Item.(*pkg.ItemMysql)
	if !ok {
		m.logger.Warn("item not support mysql")
		err = c.NextItem()
		return
	}

	if item == nil {
		err = errors.New("nil item")
		m.logger.Error(err)
		m.stats.IncItemError()
		err = c.NextItem()
		return
	}

	if item.Table == "" {
		err = errors.New("table is empty")
		m.logger.Error(err)
		m.stats.IncItemError()
		err = c.NextItem()
		return
	}

	data := item.GetData()
	if data == nil {
		err = errors.New("nil data")
		m.logger.Error(err)
		m.stats.IncItemError()
		err = c.NextItem()
		return
	}

	if m.info.Mode == "test" {
		m.logger.Debug("current mode don't need save")
		m.stats.IncItemIgnore()
		err = c.NextItem()
		return
	}

	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	refType := reflect.TypeOf(item.Data).Elem()
	refValue := reflect.ValueOf(item.Data).Elem()
	var columns []string
	var values []any
	for i := 0; i < refType.NumField(); i++ {
		column := refType.Field(i).Tag.Get("column")
		if column == "" {
			column = refType.Field(i).Name
		}
		columns = append(columns, fmt.Sprintf("%s=?", column))
		value := refValue.Field(i).Interface()
		values = append(values, value)
	}

	s := fmt.Sprintf(`INSERT %s SET %s`, item.Table, strings.Join(columns, ","))
	stmt, err := m.mysql.PrepareContext(ctx, s)
	if err != nil {
		m.logger.Error(err)
		m.stats.IncItemError()
		err = c.NextItem()
		return
	}
	res, err := stmt.ExecContext(ctx, values...)
	if err != nil {
		e, o := err.(*mysql.MySQLError)
		if !o {
			m.logger.Error(e)
			m.stats.IncItemError()
			err = c.NextItem()
			return
		}

		if item.Update && !reflect.ValueOf(item.Id).IsZero() && e.Number == 1062 {
			s = fmt.Sprintf(`UPDATE %s SET %s WHERE id=?`, item.Table, strings.Join(columns, ","))
			values = append(values, item.Id)
			stmt, err = m.mysql.PrepareContext(ctx, s)
			if err != nil {
				m.logger.Error(err)
				m.stats.IncItemError()
				err = c.NextItem()
				return
			}

			res, err = stmt.ExecContext(ctx, values...)
			if err != nil {
				m.logger.Error(err)
				m.stats.IncItemError()
				err = c.NextItem()
				return
			}

			_, err = res.RowsAffected()
			if err != nil {
				m.logger.Error(err)
				m.stats.IncItemError()
				err = c.NextItem()
				return
			}

			m.logger.Info(item.Table, "update success", item.Id)
		} else {
			m.logger.Error(err)
			m.stats.IncItemError()
			err = c.NextItem()
			return
		}
	} else {
		id, e := res.LastInsertId()
		if e != nil {
			m.logger.Error(e)
			m.stats.IncItemError()
			err = c.NextItem()
			return
		}

		m.logger.Info(item.Table, "insert success", id)
	}

	m.stats.IncItemSuccess()
	err = c.NextItem()
	return
}

func (m *MysqlMiddleware) FromCrawler(spider pkg.Spider) pkg.Middleware {
	m.logger = spider.GetLogger()
	m.mysql = spider.GetMysql()
	m.timeout = time.Minute
	return m
}

func NewMysqlMiddleware() pkg.Middleware {
	return &MysqlMiddleware{}
}
