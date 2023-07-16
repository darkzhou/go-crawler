package response

import (
	"context"
	"errors"
	"github.com/lizongying/go-crawler/pkg"
	"github.com/lizongying/go-query/query"
	"github.com/lizongying/go-re/re"
	"github.com/lizongying/go-xpath/xpath"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"time"
)

type Response struct {
	*http.Response
	request   pkg.Request
	bodyBytes []byte
	files     []pkg.File
	images    []pkg.Image
}

func (r *Response) SetResponse(response *http.Response) pkg.Response {
	r.Response = response
	return r
}
func (r *Response) GetResponse() *http.Response {
	return r.Response
}
func (r *Response) SetRequest(request pkg.Request) pkg.Response {
	r.request = request
	return r
}
func (r *Response) GetRequest() pkg.Request {
	return r.request
}
func (r *Response) SetBodyBytes(bodyBytes []byte) pkg.Response {
	r.bodyBytes = bodyBytes
	return r
}
func (r *Response) GetBodyBytes() []byte {
	return r.bodyBytes
}
func (r *Response) SetFiles(files []pkg.File) pkg.Response {
	r.files = files
	return r
}
func (r *Response) GetFiles() []pkg.File {
	return r.files
}
func (r *Response) SetImages(images []pkg.Image) pkg.Response {
	r.images = images
	return r
}
func (r *Response) GetImages() []pkg.Image {
	return r.images
}

func (r *Response) GetHeaders() http.Header {
	return r.Response.Header
}
func (r *Response) GetHeader(key string) string {
	return r.Response.Header.Get(key)
}
func (r *Response) GetStatusCode() int {
	return r.Response.StatusCode
}
func (r *Response) GetBody() io.ReadCloser {
	return r.Response.Body
}
func (r *Response) GetCookies() []*http.Cookie {
	return r.Response.Cookies()
}

func (r *Response) GetUniqueKey() string {
	return r.request.GetUniqueKey()
}
func (r *Response) UnmarshalExtra(v any) error {
	return r.request.UnmarshalExtra(v)
}
func (r *Response) GetUrl() string {
	return r.request.GetUrl()
}
func (r *Response) Context() context.Context {
	return r.request.Context()
}
func (r *Response) WithContext(ctx context.Context) pkg.Request {
	return r.request.WithContext(ctx)
}
func (r *Response) GetFile() bool {
	return r.request.GetFile()
}
func (r *Response) GetImage() bool {
	return r.request.GetImage()
}
func (r *Response) GetSkipMiddleware() bool {
	return r.request.GetSkipMiddleware()
}
func (r *Response) SetSpendTime(spendTime time.Duration) pkg.Request {
	return r.request.SetSpendTime(spendTime)
}

// Xpath returns a xpath selector
func (r *Response) Xpath() (selector *xpath.Selector, err error) {
	if r == nil {
		err = errors.New("response is invalid")
		return
	}

	if len(r.bodyBytes) == 0 {
		err = errors.New("response body is empty")
		return
	}

	selector, err = xpath.NewSelectorFromBytes(r.bodyBytes)
	if err != nil {
		return
	}

	return
}

// Query returns a query selector
func (r *Response) Query() (selector *query.Selector, err error) {
	if r == nil {
		err = errors.New("response is invalid")
		return
	}

	if len(r.bodyBytes) == 0 {
		err = errors.New("response body is empty")
		return
	}

	selector, err = query.NewSelectorFromBytes(r.bodyBytes)
	if err != nil {
		return
	}

	return
}

// Json return a gjson
func (r *Response) Json() (result gjson.Result, err error) {
	if r == nil {
		err = errors.New("response is invalid")
		return
	}

	if len(r.bodyBytes) == 0 {
		err = errors.New("response body is empty")
		return
	}

	result = gjson.ParseBytes(r.bodyBytes)

	return
}

// Re return a regex
func (r *Response) Re() (selector *re.Selector, err error) {
	if r == nil {
		err = errors.New("response is invalid")
		return
	}

	if len(r.bodyBytes) == 0 {
		err = errors.New("response body is empty")
		return
	}

	selector, err = re.NewSelectorFromBytes(r.bodyBytes)
	if err != nil {
		return
	}

	return
}