// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xio"
)

// Printer holds the information for a printer. Note that the User, Password, and UseTLS fields must be filled in if you
// wish to use those features, as the call to AvailablePrinters() will not fill them in for you.
type Printer struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	RemotePath       string   `json:"remote_path"`
	AuthInfoRequired string   `json:"auth_info_required"`
	User             string   `json:"user,omitempty"`
	Password         string   `json:"password,omitempty"`
	MimeTypes        []string `json:"mime_types,omitempty"`
	httpClient       *http.Client
	lastID           uint32
	Color            bool `json:"color,omitempty"`
	Duplex           bool `json:"duplex,omitempty"`
	UseTLS           bool `json:"use_tls,omitempty"`
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

func ippAttribute(attrs goipp.Attributes, key string) goipp.Values {
	for _, one := range attrs {
		if one.Name == key {
			return one.Values
		}
	}
	return nil
}

const (
	attrRequestedAttributes = "requested-attributes"
)

func (p *Printer) printerURI() string {
	prefix := "ipp"
	if p.UseTLS {
		prefix += "s"
	}
	return fmt.Sprintf("%s://%s:%d/%s", prefix, p.Host, p.Port, p.RemotePath)
}

// Attributes returns the printer's attributes.
func (p *Printer) Attributes(ctx context.Context) (*PrinterAttr, error) {
	p.lastID++
	req := goipp.NewRequest(goipp.DefaultVersion, goipp.OpGetPrinterAttributes, p.lastID)
	req.Operation.Add(goipp.MakeAttribute("attributes-charset", goipp.TagCharset, goipp.String("utf-8")))
	req.Operation.Add(goipp.MakeAttribute("attributes-natural-language", goipp.TagLanguage, goipp.String("en-US")))
	req.Operation.Add(goipp.MakeAttribute("requesting-user-name", goipp.TagName, goipp.String(toolbox.CurrentUserName())))
	req.Operation.Add(goipp.MakeAttribute("printer-uri", goipp.TagURI, goipp.String(p.printerURI())))
	req.Operation.Add(goipp.MakeAttribute(attrRequestedAttributes, goipp.TagKeyword, goipp.String("all")))
	rsp, err := p.sendRequest(ctx, "", req, nil, 0)
	if err != nil {
		return nil, err
	}
	if goipp.Status(rsp.Code) != goipp.StatusOk {
		msg := fmt.Sprintf(i18n.Text("Error code 0x%04x"), rsp.Code)
		if values := ippAttribute(rsp.Operation, "status-message"); values != nil {
			msg += ": " + values.String()
		}
		return nil, errs.New(msg)
	}
	// var attr PrinterAttr
	// attr.MaxCopies = 1
	// for _, a := range rsp.Printer {
	// 	switch a.Name {
	// 	case printerAttrColorModeDefault:
	// 		if a.Values[0].T == goipp.TagKeyword {
	// 			attr.ColorModeDefault = a.Values[0].V.String()
	// 		}
	// 	case printerAttrColorModeSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				attr.ColorModes = append(attr.ColorModes, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrCopiesSupported:
	// 		if a.Values[0].T == goipp.TagRange {
	// 			attr.MaxCopies = a.Values[0].V.(goipp.Range).Upper
	// 		}
	// 	case printerAttrDocumentFormatSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagMimeType {
	// 				attr.MimeTypes = append(attr.MimeTypes, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrJobCreationAttributesSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				attr.JobCreationAttributes = append(attr.JobCreationAttributes, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrMediaBottomMarginSupported:
	// 		if a.Values[0].T == goipp.TagInteger {
	// 			attr.Margin.Bottom = int(a.Values[0].V.(goipp.Integer))
	// 		}
	// 	case printerAttrMediaDefault:
	// 		if a.Values[0].T == goipp.TagKeyword {
	// 			attr.MediaDefault = a.Values[0].V.String()
	// 		}
	// 	case printerAttrMediaLeftMarginSupported:
	// 		if a.Values[0].T == goipp.TagInteger {
	// 			attr.Margin.Left = int(a.Values[0].V.(goipp.Integer))
	// 		}
	// 	case printerAttrMediaRightMarginSupported:
	// 		if a.Values[0].T == goipp.TagInteger {
	// 			attr.Margin.Right = int(a.Values[0].V.(goipp.Integer))
	// 		}
	// 	case printerAttrMediaSourceSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				attr.MediaSources = append(attr.MediaSources, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrMediaSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				attr.Media = append(attr.Media, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrMediaTopMarginSupported:
	// 		if a.Values[0].T == goipp.TagInteger {
	// 			attr.Margin.Top = int(a.Values[0].V.(goipp.Integer))
	// 		}
	// 	case printerAttrOptimizeDefault:
	// 		if a.Values[0].T == goipp.TagKeyword {
	// 			attr.OptimizeDefault = a.Values[0].V.String()
	// 		}
	// 	case printerAttrOptimizeSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				attr.Optimize = append(attr.Optimize, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrOrientationRequestedSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagEnum {
	// 				switch int(one.V.(goipp.Integer)) {
	// 				case 3:
	// 					attr.Orientations = append(attr.Orientations, Portrait)
	// 				case 4:
	// 					attr.Orientations = append(attr.Orientations, Landscape)
	// 				case 5:
	// 					attr.Orientations = append(attr.Orientations, ReverseLandscape)
	// 				case 6:
	// 					attr.Orientations = append(attr.Orientations, ReversePortrait)
	// 				}
	// 			}
	// 		}
	// 	case printerAttrIcons:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagURI {
	// 				attr.IconURLs = append(attr.IconURLs, one.V.String())
	// 			}
	// 		}
	// 	case printerAttrSidesDefault:
	// 		if a.Values[0].T == goipp.TagKeyword {
	// 			attr.SidesDefault = a.Values[0].V.String()
	// 		}
	// 	case printerAttrSidesSupported:
	// 		for _, one := range a.Values {
	// 			if one.T == goipp.TagKeyword {
	// 				switch one.V.String() {
	// 				case OneSided:
	// 					attr.Sides = append(attr.Sides, OneSided)
	// 				case TwoSidedLong:
	// 					attr.Sides = append(attr.Sides, TwoSidedLong)
	// 				case TwoSidedShort:
	// 					attr.Sides = append(attr.Sides, TwoSidedShort)
	// 				}
	// 			}
	// 		}
	// 	default:
	// 		fmt.Println(a.Name, ":", a.Values)
	// 	}
	// }
	return newPrinterAttr(rsp.Printer), nil
}

func (p *Printer) uri(namespace string) string {
	proto := "http"
	if p.UseTLS {
		proto = "https"
	}
	uri := fmt.Sprintf("%s://%s:%d", proto, p.Host, p.Port)
	//	uri := fmt.Sprintf("%s://%s:%d/%s", proto, p.Host, p.Port, p.RemotePath)
	if namespace != "" {
		uri += "/" + namespace
	}
	return uri
}

func (p *Printer) sendRequest(ctx context.Context, namespace string, req *goipp.Message, fileData io.Reader, fileLength int) (*goipp.Message, error) {
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
	if httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, p.uri(namespace), r); err != nil {
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
