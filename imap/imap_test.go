package imap

import (
	"testing"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/config"
	goimap "github.com/emersion/go-imap/v2"
)

func TestSearchCriteriaUnread(t *testing.T) {
	criteria := searchCriteria(config.Inbox{Unread: true})

	if len(criteria.NotFlag) != 1 {
		t.Fatalf("expected exactly one excluded flag, got %d", len(criteria.NotFlag))
	}
	if criteria.NotFlag[0] != goimap.FlagSeen {
		t.Fatalf("expected excluded flag %q, got %q", goimap.FlagSeen, criteria.NotFlag[0])
	}
}

func TestSearchCriteriaAgeWindow(t *testing.T) {
	criteria := searchCriteria(config.Inbox{
		MinAge: 2 * time.Hour,
		MaxAge: 24 * time.Hour,
	})

	if criteria.Before.IsZero() {
		t.Fatal("expected Before to be set")
	}
	if criteria.Since.IsZero() {
		t.Fatal("expected Since to be set")
	}
	if !criteria.Before.After(criteria.Since) {
		t.Fatalf("expected Before to be after Since, got since=%v before=%v", criteria.Since, criteria.Before)
	}

	beforeDelta := time.Since(criteria.Before)
	if beforeDelta < -(5*time.Second) || beforeDelta > 2*time.Hour+5*time.Second {
		t.Fatalf("unexpected Before delta: %v", beforeDelta)
	}

	sinceDelta := time.Since(criteria.Since)
	if sinceDelta < 24*time.Hour-5*time.Second || sinceDelta > 24*time.Hour+5*time.Second {
		t.Fatalf("unexpected Since delta: %v", sinceDelta)
	}
}