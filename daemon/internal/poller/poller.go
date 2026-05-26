package poller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v88/github"

	"github.com/sanguine59/ghlistend/daemon/internal/notifier"
	"github.com/sanguine59/ghlistend/daemon/internal/store"
)

const (
	defaultInterval = 60 * time.Second
	maxBackoff      = 5 * time.Minute
	maxPages        = 50
	requestTimeout = 30 * time.Second
)

type Options struct {
	Token          string
	NotifyExisting bool
	Logger         *slog.Logger
}

type Poller struct {
	client   *github.Client
	http     *http.Client
	store    *store.Store
	notifier *notifier.Notifier
	log      *slog.Logger
	opts     Options
}

func New(ctx context.Context, opts Options, st *store.Store, nt *notifier.Notifier) (*Poller, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	gh, err := github.NewClient(github.WithAuthToken(opts.Token))
	if err != nil {
		return nil, fmt.Errorf("github client: %w", err)
	}
	return &Poller{
		client:   gh,
		http:     gh.Client(),
		store:    st,
		notifier: nt,
		log:      opts.Logger,
		opts:     opts,
	}, nil
}

func (p *Poller) Run(ctx context.Context) error {
	cp, err := p.store.LoadCheckpoint()
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}
	hasCp, err := p.store.HasCheckpoint()
	if err != nil {
		return fmt.Errorf("has checkpoint: %w", err)
	}
	firstRun := !hasCp

	lastModified := cp.LastModified
	interval := defaultInterval
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		nextInterval, newLastModified, status, retryAfter, perr := p.pollOnce(ctx, lastModified, firstRun)
		switch {
		case perr == nil:
			backoff = time.Second
			if newLastModified != "" {
				lastModified = newLastModified
			}
			_ = p.store.SaveCheckpoint(lastModified, time.Now())
			if nextInterval > 0 {
				interval = nextInterval
			}
			firstRun = false
			p.sleep(ctx, interval)

		case errors.Is(perr, errUnauthorized):
			p.log.Error("authentication failed; stopping", "status", status)
			_ = p.notifier.Notify(notifier.Notification{
				Summary: "ghlistend: re-authentication required",
				Body:    "GitHub rejected the stored token. Run `ghlistend login` again.",
			})
			return perr

		case errors.Is(perr, errRateLimited):
			wait := retryAfter
			if wait <= 0 {
				wait = interval
			}
			p.log.Warn("rate limited", "retry_after", wait)
			p.sleep(ctx, wait)

		default:
			p.log.Warn("poll failed; backing off", "err", perr, "backoff", backoff)
			p.sleep(ctx, backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// ErrUnauthorized is returned from Run when GitHub rejects the stored token.
// Callers should treat this as a terminal, non-retryable condition: the daemon
// cannot make progress until the user re-authenticates.
var ErrUnauthorized = errors.New("unauthorized")

var (
	errUnauthorized = ErrUnauthorized
	errRateLimited  = errors.New("rate limited")
)

func (p *Poller) pollOnce(ctx context.Context, ifModSince string, firstRun bool) (nextInterval time.Duration, newLastModified string, status int, retryAfter time.Duration, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := p.client.NewRequest(reqCtx, "GET", "notifications", nil)
	if err != nil {
		return 0, "", 0, 0, fmt.Errorf("build request: %w", err)
	}
	if ifModSince != "" {
		req.Header.Set("If-Modified-Since", ifModSince)
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return 0, "", 0, 0, err
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	status = resp.StatusCode
	nextInterval = parsePollInterval(resp.Header.Get("X-Poll-Interval"))
	newLastModified = resp.Header.Get("Last-Modified")

	switch status {
	case http.StatusNotModified:
		_, _ = io.Copy(io.Discard, resp.Body)
		p.log.Debug("no changes", "interval", nextInterval)
		return nextInterval, newLastModified, status, 0, nil

	case http.StatusOK:
		var notifications []*github.Notification
		if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
			return nextInterval, newLastModified, status, 0, fmt.Errorf("decode body: %w", err)
		}

		for page := 1; page < maxPages; page++ {
			next := parseLinkNext(resp.Header.Get("Link"))
			if next == "" {
				break
			}
			resp.Body.Close()
			resp = nil
			pageCtx, pageCancel := context.WithTimeout(ctx, requestTimeout)
			nxtreq, err := http.NewRequestWithContext(pageCtx, http.MethodGet, next, nil)
			if err != nil {
				pageCancel()
				p.log.Warn("page: couldnt build request", "err", err)
				break
			}
			nxtreq.Header.Set("Authorization", req.Header.Get("Authorization"))

			resp, err = p.http.Do(nxtreq)
			if err != nil {
				pageCancel()
				p.log.Warn("page: request failed", "err", err)
				break
			}
			
			defer pageCancel()
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
				ra := parseRetryAfter(resp.Header.Get("Retry-After"))
				resp.Body.Close()
				resp = nil
				return nextInterval, "", http.StatusTooManyRequests, ra, errRateLimited
			}
			if resp.StatusCode != http.StatusOK {
				p.log.Warn("page: status not expected", "status", resp.StatusCode)
				resp.Body.Close()
				break
			}

			var nextPage []*github.Notification
			if err := json.NewDecoder(resp.Body).Decode(&nextPage); err != nil {
				resp.Body.Close()
				p.log.Warn("page: decode failed", "err", err)
				resp = nil
				break
			}
			notifications = append(notifications, nextPage...)
		}

		p.log.Debug("received notifications", "count", len(notifications))
		if firstRun && !p.opts.NotifyExisting {
			for _, n := range notifications {
				_ = p.store.MarkSeen(n.GetID(), n.GetUpdatedAt().UTC().Format(time.RFC3339))
			}
			p.log.Info("first-run priming complete", "primed", len(notifications))
			return nextInterval, newLastModified, status, 0, nil
		}
		for _, n := range notifications {
			updatedAt := n.GetUpdatedAt().UTC().Format(time.RFC3339)
			seen, err := p.store.Seen(n.GetID(), updatedAt)
			if err != nil {
				p.log.Warn("store seen check failed", "err", err)
				continue
			}
			if seen {
				continue
			}
			p.dispatch(n)
			_ = p.store.MarkSeen(n.GetID(), updatedAt)
		}
		return nextInterval, newLastModified, status, 0, nil

	case http.StatusUnauthorized:
		return 0, "", status, 0, errUnauthorized

	case http.StatusForbidden, http.StatusTooManyRequests:
		retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		return nextInterval, "", status, retryAfter, errRateLimited

	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nextInterval, "", status, 0, fmt.Errorf("unexpected status %d: %s", status, string(body))
	}
}

func (p *Poller) dispatch(n *github.Notification) {
	repo := ""
	if r := n.GetRepository(); r != nil {
		repo = r.GetFullName()
	}
	subject := n.GetSubject()
	title := ""
	if subject != nil {
		title = subject.GetTitle()
	}
	body := title
	if reason := n.GetReason(); reason != "" {
		body = fmt.Sprintf("%s\n(%s)", title, reason)
	}
	if err := p.notifier.Notify(notifier.Notification{
		Summary: repo,
		Body:    body,
	}); err != nil {
		p.log.Warn("notify failed", "err", err, "thread", n.GetID())
	}
}

func (p *Poller) sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func parseLinkNext(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		seg := strings.SplitN(part, ";", 2)
		if len(seg) < 2 {
			continue
		}
		url := strings.Trim(strings.TrimSpace(seg[0]), "<>")
		for _, attr := range strings.Split(seg[1], ";") {
			if strings.TrimSpace(attr) == `rel="next"` {
				return url
			}
		}
	}
	return ""
}

func parsePollInterval(h string) time.Duration {
	if h == "" {
		return 0
	}
	n, err := strconv.Atoi(h)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
