// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xio"
)

// Printer holds the information for a printer. Note that the User, Password, and UseTLS fields must be filled in if you
// wish to use those features, as the call to PrintManager.Printers() will not fill them in for you.
type Printer struct {
	ID               string
	Name             string
	Host             string
	Port             int
	RemotePath       string
	AuthInfoRequired string
	User             string
	Password         string
	MimeTypes        []string
	Color            bool
	Duplex           bool
	UseTLS           bool
	lastID           uint32
	httpClient       *http.Client
	lock             sync.RWMutex
	attributes       *PrinterAttributes
}

// MimeTypeSupported returns true if the given MIME type is supported by the printer.
func (p *Printer) MimeTypeSupported(mimeType string) bool {
	for _, mt := range p.MimeTypes {
		if mt == mimeType {
			return true
		}
	}
	return false
}

// Attributes returns the printer's attributes. If allowCachedReturn is true and a previous call to Attributes() was
// made successfully, the previous data will be returned instead of communicating with the printer again.
func (p *Printer) Attributes(timeout time.Duration, allowCachedReturn bool) (*PrinterAttributes, error) {
	if allowCachedReturn {
		p.lock.RLock()
		if p.attributes != nil {
			defer p.lock.RUnlock()
			return p.attributes, nil
		}
		p.lock.RUnlock()
	}
	if timeout < 1 {
		return nil, context.DeadlineExceeded
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.lastID++
	req := p.newRequest(p.lastID, goipp.OpGetPrinterAttributes)
	req.Operation.Add(goipp.MakeAttribute("requested-attributes", goipp.TagKeyword, goipp.String("all")))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rsp, err := p.sendRequest(ctx, req, nil, 0)
	if err != nil {
		return nil, err
	}
	if err = checkIPPStatus(rsp); err != nil {
		return nil, err
	}
	p.attributes = NewAttributes(rsp.Printer).ForPrinter()
	fmt.Println("==== SUPPORTED with DEFAULT =====")
	for _, one := range rsp.Printer {
		if strings.HasSuffix(one.Name, "-supported") {
			v, ok := p.attributes.Attributes[strings.TrimSuffix(one.Name, "-supported")+"-default"]
			if ok {
				fmt.Printf("%s: [%v]: %v\n", one.Name, v, one.Values)
			}
		}
	}
	fmt.Println("\n==== SUPPORTED without DEFAULT =====")
	for _, one := range rsp.Printer {
		if strings.HasSuffix(one.Name, "-supported") {
			_, ok := p.attributes.Attributes[strings.TrimSuffix(one.Name, "-supported")+"-default"]
			if !ok {
				fmt.Printf("%s: %v\n", one.Name, one.Values)
			}
		}
	}
	return p.attributes, nil
}

// Validate the job attributes. Any attributes that have been either ignored or altered by the printer will be returned.
func (p *Printer) Validate(ctx context.Context, jobName, mimeType string, attributes *JobAttributes) (*JobAttributes, error) {
	p.lock.Lock()
	p.lastID++
	id := p.lastID
	p.lock.Unlock()
	req := p.newRequest(id, goipp.OpValidateJob)
	addAttributesForJob(req, jobName, mimeType)
	req.Job = attributes.toIPP()
	rsp, err := p.sendRequest(ctx, req, nil, 0)
	if err != nil {
		return nil, err
	}
	if err = checkIPPStatus(rsp); err != nil {
		return nil, err
	}
	return NewAttributes(rsp.Unsupported).ForJob(), nil
}

// Print a document.
func (p *Printer) Print(ctx context.Context, jobName, mimeType string, fileData io.Reader, fileLength int, attributes *JobAttributes) error {
	p.lock.Lock()
	p.lastID++
	id := p.lastID
	p.lock.Unlock()
	req := p.newRequest(id, goipp.OpPrintJob)
	addAttributesForJob(req, jobName, mimeType)
	req.Job = attributes.toIPP()
	rsp, err := p.sendRequest(ctx, req, fileData, fileLength)
	if err != nil {
		return err
	}
	if err = checkIPPStatus(rsp); err != nil {
		return err
	}
	return nil
}

func checkIPPStatus(rsp *goipp.Message) error {
	if rsp.Code <= 0xFF {
		return nil
	}
	msg := fmt.Sprintf(i18n.Text("Error code 0x%04x"), rsp.Code)
	if s := NewAttributes(rsp.Operation).Strings("status-message", nil); s != nil {
		msg += ":\n" + strings.Join(s, "\n")
	}
	return errs.New(msg)
}

func (p *Printer) useTLS() string {
	if p.UseTLS {
		return "s"
	}
	return ""
}

func (p *Printer) uri() string {
	return fmt.Sprintf("http%s://%s:%d/%s", p.useTLS(), p.Host, p.Port, p.RemotePath)
}

func (p *Printer) printerURI() string {
	return fmt.Sprintf("ipp%s://%s:%d/%s", p.useTLS(), p.Host, p.Port, p.RemotePath)
}

func (p *Printer) newRequest(id uint32, op goipp.Op) *goipp.Message {
	req := goipp.NewRequest(goipp.DefaultVersion, op, id)
	req.Operation.Add(goipp.MakeAttribute("printer-uri", goipp.TagURI, goipp.String(p.printerURI())))
	req.Operation.Add(goipp.MakeAttribute("requesting-user-name", goipp.TagName, goipp.String(toolbox.CurrentUserName())))
	req.Operation.Add(goipp.MakeAttribute("attributes-charset", goipp.TagCharset, goipp.String("utf-8")))
	req.Operation.Add(goipp.MakeAttribute("attributes-natural-language", goipp.TagLanguage, goipp.String("en-US")))
	return req
}

func addAttributesForJob(req *goipp.Message, jobName, mimeType string) {
	req.Operation.Add(goipp.MakeAttribute("document-format", goipp.TagMimeType, goipp.String(mimeType)))
	req.Operation.Add(goipp.MakeAttribute("job-name", goipp.TagName, goipp.String(jobName)))
}

func (p *Printer) sendRequest(ctx context.Context, req *goipp.Message, fileData io.Reader, fileLength int) (*goipp.Message, error) {
	data, err := req.EncodeBytes()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	var r io.Reader
	r = bytes.NewReader(data)
	if fileData != nil {
		r = io.MultiReader(r, fileData)
	}
	var httpReq *http.Request
	if httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, p.uri(), r); err != nil {
		return nil, errs.Wrap(err)
	}
	httpReq.Header.Set("Content-Length", strconv.Itoa(len(data)+fileLength))
	httpReq.Header.Set("Content-Type", goipp.ContentType)
	if p.User != "" && p.Password != "" {
		httpReq.SetBasicAuth(p.User, p.Password)
	}
	var httpResp *http.Response
	if httpResp, err = p.httpClient.Do(httpReq); err != nil { //nolint:bodyclose // xio.DiscardAndCloseIgnoringErrors does this
		return nil, errs.Wrap(err)
	}
	defer xio.DiscardAndCloseIgnoringErrors(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return nil, errs.Newf("unexpected http response code: %d", httpResp.StatusCode)
	}
	rsp := goipp.NewResponse(0, 0, 0)
	if err = rsp.Decode(httpResp.Body); err != nil {
		return nil, errs.Wrap(err)
	}
	return rsp, nil
}
