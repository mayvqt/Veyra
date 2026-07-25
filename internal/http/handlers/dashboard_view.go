package handlers

func dashboardServices(mediaServerName, mediaServerURL, mediaServerStatus, seerrURL, seerrStatus string) []DashboardService {
	services := make([]DashboardService, 0, 2)
	if mediaServerURL != "" {
		services = append(services, DashboardService{
			Name:   mediaServerName,
			URL:    mediaServerURL,
			Status: mediaServerStatus,
		})
	}
	if seerrURL != "" {
		services = append(services, DashboardService{
			Name:   "Seerr",
			URL:    seerrURL,
			Status: seerrStatus,
		})
	}
	return services
}
