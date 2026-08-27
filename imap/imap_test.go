package imap

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dominicgisler/imap-spam-cleaner/app"
	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func TestSearchCriteriaUnread(t *testing.T) {
	criteria := searchCriteria(app.Inbox{Unread: true})

	if len(criteria.NotFlag) != 1 {
		t.Fatalf("expected exactly one excluded flag, got %d", len(criteria.NotFlag))
	}
	if criteria.NotFlag[0] != goimap.FlagSeen {
		t.Fatalf("expected excluded flag %q, got %q", goimap.FlagSeen, criteria.NotFlag[0])
	}
}

func TestSearchCriteriaAgeWindow(t *testing.T) {
	criteria := searchCriteria(app.Inbox{
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

func TestMoveMessageMarkReadOption(t *testing.T) {
	tests := []struct {
		name             string
		markRead         bool
		expectedCommands []string
	}{
		{
			name:     "enabled",
			markRead: true,
			expectedCommands: []string{
				" UID STORE 42 +FLAGS.SILENT (\\Seen)",
				` UID MOVE 42 "Spam"`,
			},
		},
		{
			name:             "disabled",
			markRead:         false,
			expectedCommands: []string{` UID MOVE 42 "Spam"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			t.Cleanup(func() {
				_ = clientConn.Close()
				_ = serverConn.Close()
			})

			serverDone := make(chan error, 1)
			go func() {
				defer serverConn.Close()

				reader := bufio.NewReader(serverConn)
				writer := bufio.NewWriter(serverConn)

				if _, err := writer.WriteString("* OK [CAPABILITY IMAP4rev1 MOVE] ready\r\n"); err != nil {
					serverDone <- err
					return
				}
				if err := writer.Flush(); err != nil {
					serverDone <- err
					return
				}

				for _, expected := range test.expectedCommands {
					line, err := reader.ReadString('\n')
					if err != nil {
						serverDone <- err
						return
					}
					if !strings.Contains(line, expected) {
						serverDone <- fmt.Errorf("expected command containing %q, got %q", expected, strings.TrimSpace(line))
						return
					}

					tag := strings.Fields(line)[0]
					if _, err := writer.WriteString(tag + " OK command completed\r\n"); err != nil {
						serverDone <- err
						return
					}
					if err := writer.Flush(); err != nil {
						serverDone <- err
						return
					}
				}

				serverDone <- nil
			}()

			client := imapclient.New(clientConn, nil)
			if err := client.WaitGreeting(); err != nil {
				t.Fatalf("expected greeting to succeed: %v", err)
			}

			im := &Imap{client: client}
			moveErr := im.MoveMessage(42, "Spam", test.markRead)
			if err := <-serverDone; err != nil {
				t.Fatal(err)
			}
			if moveErr != nil {
				t.Fatalf("expected move to succeed: %v", moveErr)
			}
		})
	}
}

func TestCloseLogsOutGracefully(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		writer := bufio.NewWriter(serverConn)

		if _, err := writer.WriteString("* OK ready\r\n"); err != nil {
			serverDone <- err
			return
		}
		if err := writer.Flush(); err != nil {
			serverDone <- err
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			serverDone <- err
			return
		}
		if !strings.Contains(line, " LOGOUT") {
			serverDone <- fmt.Errorf("expected LOGOUT command, got %q", strings.TrimSpace(line))
			return
		}

		tag := strings.Fields(line)[0]
		if _, err := writer.WriteString("* BYE logging out\r\n" + tag + " OK logout completed\r\n"); err != nil {
			serverDone <- err
			return
		}
		if err := writer.Flush(); err != nil {
			serverDone <- err
			return
		}

		serverDone <- nil
	}()

	client := imapclient.New(clientConn, nil)
	if err := client.WaitGreeting(); err != nil {
		t.Fatalf("expected greeting to succeed: %v", err)
	}

	im := &Imap{client: client}
	if err := im.Close(); err != nil {
		t.Fatalf("expected graceful close, got %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	if im.client != nil {
		t.Fatal("expected client to be cleared after close")
	}
}

func TestNewClosesConnectionWhenListFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			serverDone <- err
			return
		}

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		if _, err := writer.WriteString("* OK ready\r\n"); err != nil {
			serverDone <- err
			return
		}
		if err := writer.Flush(); err != nil {
			serverDone <- err
			return
		}

		loginLine, err := reader.ReadString('\n')
		if err != nil {
			serverDone <- err
			return
		}
		if !strings.Contains(loginLine, " LOGIN ") {
			serverDone <- fmt.Errorf("expected LOGIN command, got %q", strings.TrimSpace(loginLine))
			return
		}
		loginTag := strings.Fields(loginLine)[0]
		if _, err := writer.WriteString(loginTag + " OK LOGIN completed\r\n"); err != nil {
			serverDone <- err
			return
		}
		if err := writer.Flush(); err != nil {
			serverDone <- err
			return
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}

			tag := strings.Fields(line)[0]
			switch {
			case strings.Contains(line, " CAPABILITY"):
				if _, err := writer.WriteString("* CAPABILITY IMAP4rev1\r\n" + tag + " OK CAPABILITY completed\r\n"); err != nil {
					serverDone <- err
					return
				}
				if err := writer.Flush(); err != nil {
					serverDone <- err
					return
				}
			case strings.Contains(line, " LIST "):
				if _, err := writer.WriteString(tag + " BAD LIST failed\r\n"); err != nil {
					serverDone <- err
					return
				}
				if err := writer.Flush(); err != nil {
					serverDone <- err
					return
				}
				goto waitForLogout
			default:
				serverDone <- fmt.Errorf("expected CAPABILITY or LIST command, got %q", strings.TrimSpace(line))
				return
			}
		}

	waitForLogout:
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}
			tag := strings.Fields(line)[0]
			switch {
			case strings.Contains(line, " CAPABILITY"):
				if _, err := writer.WriteString("* CAPABILITY IMAP4rev1\r\n" + tag + " OK CAPABILITY completed\r\n"); err != nil {
					serverDone <- err
					return
				}
			case strings.Contains(line, " LOGOUT"):
				if _, err := writer.WriteString("* BYE logging out\r\n" + tag + " OK logout completed\r\n"); err != nil {
					serverDone <- err
					return
				}
				if err := writer.Flush(); err != nil {
					serverDone <- err
					return
				}
				serverDone <- nil
				return
			default:
				serverDone <- fmt.Errorf("expected CAPABILITY or LOGOUT after LIST failure, got %q", strings.TrimSpace(line))
				return
			}
			if err := writer.Flush(); err != nil {
				serverDone <- err
				return
			}
		}
	}()

	_, err = New(app.Inbox{
		Host:     "127.0.0.1",
		Port:     listener.Addr().(*net.TCPAddr).Port,
		Username: "user@example.com",
		Password: "secret",
		Inbox:    "INBOX",
	})
	if err == nil {
		t.Fatal("expected mailbox list failure")
	}
	if !strings.Contains(err.Error(), "failed to list mailboxes") {
		t.Fatalf("expected mailbox list error, got %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}
