// Package views retrieves data and controls which of it is visible.
//
// This is the only package that should interact directly with Twilio - all
// other code should talk to this package to determine whether a particular
// piece of information should be visible, or not.
package views

import (
	"net/http"
	"net/url"

	log "github.com/inconshreveable/log15"
	twilio "github.com/kevinburke/twilio-go"
	"github.com/saintpete/logrole/config"
	"github.com/saintpete/logrole/services"
)

// A Client retrieves resources from the Twilio API, and hides information that
// shouldn't be seen before returning them to the caller.
type Client struct {
	log.Logger
	client     *twilio.Client
	secretKey  *[32]byte
	permission *config.Permission
}

// NewClient creates a new Client encapsulating the provided values.
func NewClient(l log.Logger, client *twilio.Client, secretKey *[32]byte, p *config.Permission) *Client {
	return &Client{
		Logger:     l,
		client:     client,
		secretKey:  secretKey,
		permission: p,
	}
}

// SetBasicAuth sets the Twilio AccountSid and AuthToken on the given request.
func (vc *Client) SetBasicAuth(r *http.Request) {
	r.SetBasicAuth(vc.client.AccountSid, vc.client.AuthToken)
}

// GetMessage fetches a single Message from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *Client) GetMessage(user *config.User, sid string) (*Message, error) {
	message, err := vc.client.Messages.Get(sid)
	if err != nil {
		return nil, err
	}
	return NewMessage(message, vc.permission, user)
}

// GetCall fetches a single Call from the Twilio API, and returns any
// network or permission errors that occur.
func (vc *Client) GetCall(user *config.User, sid string) (*Call, error) {
	call, err := vc.client.Calls.Get(sid)
	if err != nil {
		return nil, err
	}
	return NewCall(call, vc.permission, user)
}

// Just make sure we get all of the media when we make a request
var mediaUrlsFilters = url.Values{
	"PageSize": []string{"100"},
}

// GetMediaURLs retrieves all media URL's for a given client, but encrypts and
// obscures them behind our image proxy first.
func (vc *Client) GetMediaURLs(u *config.User, sid string) ([]*url.URL, error) {
	if u.CanViewMedia() == false {
		return nil, config.PermissionDenied
	}
	urls, err := vc.client.Messages.GetMediaURLs(sid, mediaUrlsFilters)
	if err != nil {
		return nil, err
	}
	opaqueImages := make([]*url.URL, len(urls))
	for i, u := range urls {
		enc := services.Opaque(u.String(), vc.secretKey)
		opaqueURL, err := url.Parse("/images/" + enc)
		if err != nil {
			return nil, err
		}
		opaqueImages[i] = opaqueURL
	}
	return opaqueImages, nil
}

func (vc *Client) GetMessagePage(user *config.User, data url.Values) (*MessagePage, error) {
	page, err := vc.client.Messages.GetPage(data)
	if err != nil {
		return nil, err
	}
	return NewMessagePage(page, vc.permission, user)
}

func (vc *Client) GetNextMessagePage(user *config.User, nextPage string) (*MessagePage, error) {
	page := new(twilio.MessagePage)
	err := vc.client.GetNextPage(nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewMessagePage(page, vc.permission, user)
}

func (vc *Client) GetCallPage(user *config.User, data url.Values) (*CallPage, error) {
	page, err := vc.client.Calls.GetPage(data)
	if err != nil {
		return nil, err
	}
	return NewCallPage(page, vc.permission, user)
}

func (vc *Client) GetNextCallPage(user *config.User, nextPage string) (*CallPage, error) {
	page := new(twilio.CallPage)
	err := vc.client.GetNextPage(nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewCallPage(page, vc.permission, user)
}

func (vc *Client) GetNextRecordingPage(user *config.User, nextPage string) (*RecordingPage, error) {
	page := new(twilio.RecordingPage)
	err := vc.client.GetNextPage(nextPage, page)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}

func (vc *Client) GetCallRecordings(user *config.User, callSid string, data url.Values) (*RecordingPage, error) {
	page, err := vc.client.Calls.GetRecordings(callSid, data)
	if err != nil {
		return nil, err
	}
	return NewRecordingPage(page, vc.permission, user, vc.secretKey)
}
