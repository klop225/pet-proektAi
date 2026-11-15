package main

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fakeModerator struct {
	verdict Verdict
	err     error
	check   bool
	text    string
}

func (f *fakeModerator) Moderate(ctx context.Context, comment string) (Verdict, error) {
	f.check = true
	f.text = comment
	return f.verdict, f.err
}

func newServer(m Moderator) *Server {
	t := template.Must(template.New("r").Parse(`{{.Text}} ({{.Verdict}})`))
	return &Server{mod: m, tmpl: t}
}

func post(t *testing.T, srv *Server, comment string) *httptest.ResponseRecorder {
	t.Helper()
	data := url.Values{"comment": {comment}}
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.PostHandle(rec, req)
	return rec
}

func get(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/submit", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.PostHandle(rec, req)
	return rec
}

func TestGoodComment(t *testing.T) {
	modF := &fakeModerator{verdict: Good}
	srv := newServer(modF)

	rec := post(t, srv, "привет хорошая статья")
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "хороший комментарий") {
		t.Errorf("в ответе нет вердикта: %q", rec.Body.String())
	}
}

func TestBadComment(t *testing.T) {
	modF := &fakeModerator{verdict: Bad}
	srv := newServer(modF)

	rec := post(t, srv, "ты глупый")
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "плохой комментарий") {
		t.Errorf("в ответе нет вердикта: %q", rec.Body.String())
	}
}

func TestEmptyComment(t *testing.T) {
	modF := &fakeModerator{}
	srv := newServer(modF)

	rec := post(t, srv, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("код %d, ожидали 400", rec.Code)
	}
	if modF.check {
		t.Error("модератор не должен вызываться для пустого комментария")
	}
}

func TestErrorMaxRune(t *testing.T) {
	modF := &fakeModerator{}
	srv := newServer(modF)
	comment := strings.Repeat("я", 1001)

	rec := post(t, srv, comment)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("код %d, ожидали 400", rec.Code)
	}
	if modF.check {
		t.Error("модератор не должен вызываться для слишком длинного комментария")
	}
}

func TestMaxRune(t *testing.T) {
	modF := &fakeModerator{verdict: Good}
	srv := newServer(modF)
	comment := strings.Repeat("я", 1000)

	rec := post(t, srv, comment)
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}
	if !modF.check {
		t.Error("модератор должен вызываться")
	}
}

func TestModerator(t *testing.T) {
	modF := &fakeModerator{err: errors.New(http.StatusText(http.StatusInternalServerError))}
	srv := newServer(modF)

	rec := post(t, srv, "норм")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("код %d, ожидали 500", rec.Code)
	}
}

func TestNoGet(t *testing.T) {
	modF := &fakeModerator{}
	srv := newServer(modF)

	rec := get(t, srv)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("код %d, ожидали 405", rec.Code)
	}
	if modF.check {
		t.Error("модератор не должен вызываться")
	}
}

func TestTmpl(t *testing.T) {
	modF := &fakeModerator{}
	srv := newServer(modF)

	rec := post(t, srv, "<script>alert(1)</script>")
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, ожидали 200", rec.Code)
	}

	s := rec.Body.String()
	if strings.Contains(s, "<script>") {
		t.Errorf("ошибка шаблона %q", s)
	}
	if !strings.Contains(s, "&lt;script&gt;") {
		t.Errorf("шаблон пустой %q", s)
	}
}
