package trade

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const testIdentitySecret = "AAAAAAAAAAAAAAAAAAAAAA=="

func TestGenerateConfirmationKey(t *testing.T) {
	key, ts, err := generateConfirmationKey(testIdentitySecret, "getlist", time.Unix(1, 0))
	if err != nil {
		t.Fatalf("generateConfirmationKey() error: %v", err)
	}
	if ts != 1 {
		t.Fatalf("timestamp = %d", ts)
	}
	if key != "FgNQtTcG5vN+RUyz6VZj53d0iT8=" {
		t.Fatalf("key = %q", key)
	}
}

func TestPendingConfirmations(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/mobileconf/getlist" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("p") != "android:test" {
			t.Fatalf("device id = %q", q.Get("p"))
		}
		if q.Get("a") != "76561198012345678" {
			t.Fatalf("steam id = %q", q.Get("a"))
		}
		if q.Get("tag") != "getlist" {
			t.Fatalf("tag = %q", q.Get("tag"))
		}
		return jsonResponse(`{"success":true,"conf":[{"id":"1","nonce":"nonce","creator_id":"99","creation_time":1700000000,"type":2,"accept":"accept-key","cancel":"cancel-key","icon":"icon","multi":false,"headline":"Trade","summary":["one"],"warn":""}]}`), nil
	})}

	client := newTestClient(t, httpClient)
	confs, err := client.PendingConfirmationsWithDevice(context.Background(), testIdentitySecret, "android:test")
	if err != nil {
		t.Fatalf("PendingConfirmationsWithDevice() error: %v", err)
	}
	if len(confs) != 1 {
		t.Fatalf("len(confs) = %d", len(confs))
	}
	if confs[0].ID != 1 || confs[0].CreatorID != 99 || confs[0].Type != ConfirmationTypeTrade {
		t.Fatalf("confirmation = %#v", confs[0])
	}
}

func TestConfirmOffer(t *testing.T) {
	var requests []url.Values
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Query())
		switch r.URL.Path {
		case "/mobileconf/getlist":
			return jsonResponse(`{"success":true,"conf":[{"id":"10","nonce":"nonce","creator_id":"777","creation_time":1700000000,"type":2,"accept":"accept-key","cancel":"cancel-key","icon":"","multi":false,"headline":"Trade","summary":[],"warn":""}]}`), nil
		case "/mobileconf/ajaxop":
			q := r.URL.Query()
			if q.Get("op") != "allow" {
				t.Fatalf("op = %q", q.Get("op"))
			}
			if q.Get("cid") != "10" {
				t.Fatalf("cid = %q", q.Get("cid"))
			}
			if q.Get("ck") != "accept-key" {
				t.Fatalf("ck = %q", q.Get("ck"))
			}
			return jsonResponse(`{"success":true}`), nil
		default:
			t.Fatalf("path = %q", r.URL.Path)
			return nil, nil
		}
	})}

	client := newTestClient(t, httpClient)
	conf, err := client.ConfirmOfferWithDevice(context.Background(), 777, testIdentitySecret, "android:test")
	if err != nil {
		t.Fatalf("ConfirmOfferWithDevice() error: %v", err)
	}
	if conf.ID != 10 {
		t.Fatalf("confirmation id = %d", conf.ID)
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d", len(requests))
	}
	if requests[0].Get("p") != requests[1].Get("p") {
		t.Fatalf("device id changed between requests: %q vs %q", requests[0].Get("p"), requests[1].Get("p"))
	}
}

func TestConfirmOfferNotFound(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"success":true,"conf":[]}`), nil
	})}

	client := newTestClient(t, httpClient)
	_, err := client.ConfirmOfferWithDevice(context.Background(), 777, testIdentitySecret, "android:test")
	if !errors.Is(err, ErrConfirmationNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestPendingConfirmationsNeedAuth(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"success":false,"needauth":true}`), nil
	})}

	client := newTestClient(t, httpClient)
	_, err := client.PendingConfirmationsWithDevice(context.Background(), testIdentitySecret, "android:test")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v", err)
	}
}
