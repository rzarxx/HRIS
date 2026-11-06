package attendance

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

func TestDeviceOfRejectsHeaderDifferentFromMobileSession(t *testing.T) {
	s := &Service{}
	p := auth.Principal{DeviceID: uuid.New(), EmployeeID: uuid.New()}
	_, err := s.deviceOf(context.Background(), p, uuid.NewString())
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden || apiErr.Code != "DEVICE_NOT_REGISTERED" {
		t.Fatalf("perangkat lain harus ditolak sebelum query: %v", err)
	}
}

func TestDeviceOfRejectsWebSessionOutsideDevelopment(t *testing.T) {
	s := &Service{}
	_, err := s.deviceOf(context.Background(), auth.Principal{EmployeeID: uuid.New()}, uuid.NewString())
	var apiErr *httpx.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden || apiErr.Code != "DEVICE_NOT_REGISTERED" {
		t.Fatalf("sesi web harus ditolak sebelum query: %v", err)
	}
}
