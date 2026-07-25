package handlers

import "testing"

func TestDashboardServicesOmitsUnconfiguredServices(t *testing.T) {
	services := dashboardServices("Jellyfin", "https://watch.example", "Online", "", "Offline")
	if len(services) != 1 {
		t.Fatalf("expected one configured service, got %d", len(services))
	}
	if services[0].Name != "Jellyfin" || services[0].URL != "https://watch.example" || services[0].Status != "Online" {
		t.Fatalf("unexpected dashboard service: %+v", services[0])
	}
}
