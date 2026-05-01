package models

import "fmt"

// Link represents a HATEOAS link
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
}

// Links represents a collection of HATEOAS links
type Links map[string]Link

// SubscriberWithLinks represents a subscriber with HATEOAS links
type SubscriberWithLinks struct {
	Subscriber
	Links Links `json:"_links"`
}

// SubscriberListWithLinks represents a subscriber list with HATEOAS links
type SubscriberListWithLinks struct {
	Subscribers []SubscriberWithLinks `json:"subscribers"`
	NextCursor  string                `json:"next_cursor,omitempty"`
	HasMore     bool                  `json:"has_more"`
	Links       Links                 `json:"_links"`
}

// NewSubscriberLinks creates HATEOAS links for a subscriber
func NewSubscriberLinks(baseURL string, subscriber *Subscriber) Links {
	return Links{
		"self": {
			Href: fmt.Sprintf("%s/api/v1/subscribers/%d", baseURL, subscriber.ID),
			Rel:  "self",
		},
		"update": {
			Href: fmt.Sprintf("%s/api/v1/subscribers/%d", baseURL, subscriber.ID),
			Rel:  "update",
			Type: "PUT",
		},
		"delete": {
			Href: fmt.Sprintf("%s/api/v1/subscribers/%d", baseURL, subscriber.ID),
			Rel:  "delete",
			Type: "DELETE",
		},
		"billing": {
			Href: fmt.Sprintf("%s/api/v1/subscribers/%d/billing", baseURL, subscriber.ID),
			Rel:  "billing",
		},
		"sessions": {
			Href: fmt.Sprintf("%s/api/v1/subscribers/%d/sessions", baseURL, subscriber.ID),
			Rel:  "sessions",
		},
	}
}

// NewSubscriberListLinks creates HATEOAS links for subscriber list
func NewSubscriberListLinks(baseURL string, cursor string, limit int, hasMore bool) Links {
	links := Links{
		"self": {
			Href: fmt.Sprintf("%s/api/v1/subscribers?limit=%d", baseURL, limit),
			Rel:  "self",
		},
		"create": {
			Href: fmt.Sprintf("%s/api/v1/subscribers", baseURL),
			Rel:  "create",
			Type: "POST",
		},
	}

	if cursor != "" {
		links["next"] = Link{
			Href: fmt.Sprintf("%s/api/v1/subscribers?cursor=%s&limit=%d", baseURL, cursor, limit),
			Rel:  "next",
		}
	}

	return links
}
