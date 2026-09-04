(function (dashboard) {
  var refreshIntervalMs = 45000;
  var returnRefreshDelayMs = 250;
  var refreshInFlight = false;

  function dashboardIsSafeToRefresh() {
    if (document.hidden) {
      return false;
    }
    var requestQuery = document.querySelector("[data-seerr-query]");
    if (requestQuery && requestQuery.value.trim() !== "") {
      return false;
    }
    var active = document.activeElement;
    return !active || active === document.body || active === document.documentElement;
  }

  function refreshDashboardContent() {
    if (refreshInFlight) {
      return;
    }
    if (!dashboardIsSafeToRefresh()) {
      return;
    }
    refreshInFlight = true;
    fetch(window.location.href, {
      headers: { "X-Requested-With": "fetch" },
      credentials: "same-origin",
      cache: "no-store"
    }).then(function (response) {
      if (!response.ok) {
        return "";
      }
      return response.text();
    }).then(function (html) {
      if (!html) {
        return;
      }
      var parser = new DOMParser();
      var doc = parser.parseFromString(html, "text/html");
      var currentWidgets = dashboard.queryAll("[data-dashboard-refresh]");
      if (currentWidgets.length === 0) {
        return;
      }
      if (!dashboardIsSafeToRefresh()) {
        return;
      }
      currentWidgets.forEach(function (current) {
        var key = current.getAttribute("data-dashboard-refresh");
        var next = doc.querySelector('[data-dashboard-refresh="' + key + '"]');
        if (next) {
          var scrollLeft = current.scrollLeft;
          var scrollTop = current.scrollTop;
          dashboard.replaceNodeContent(current, next);
          current.scrollLeft = scrollLeft;
          current.scrollTop = scrollTop;
        }
      });
      dashboard.initWidgets();
    }).catch(function () {}).then(function () {
      refreshInFlight = false;
    });
  }

  dashboard.initAutoRefresh = function () {
    if (window.__veyraDashboardRefreshStarted) {
      return;
    }
    window.__veyraDashboardRefreshStarted = true;
    window.setInterval(refreshDashboardContent, refreshIntervalMs);
    document.addEventListener("visibilitychange", function () {
      if (!document.hidden) {
        window.setTimeout(refreshDashboardContent, returnRefreshDelayMs);
      }
    });
  };
})(window.VeyraDashboard);
