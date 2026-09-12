(function (dashboard) {
  var refreshIntervalMs = 45000;
  var returnRefreshDelayMs = 250;
  var refreshInFlight = false;

  function dashboardIsSafeToRefresh() {
    if (document.hidden) {
      return false;
    }
    return true;
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
          var focusedFilter = null;
          if (current.contains(document.activeElement)) {
            focusedFilter = document.activeElement.getAttribute("data-kind-filter");
            if (!focusedFilter) {
              return;
            }
          }
          var scrollLeft = current.scrollLeft;
          var scrollTop = current.scrollTop;
          dashboard.replaceNodeContent(current, next);
          current.scrollLeft = scrollLeft;
          current.scrollTop = scrollTop;
          if (focusedFilter) {
            var button = current.querySelector('[data-kind-filter="' + focusedFilter + '"]');
            if (button) {
              button.focus({ preventScroll: true });
            }
          }
        }
      });
      var requestBot = document.querySelector("[data-seerr-request-bot]");
      var nextBot = doc.querySelector("[data-seerr-request-bot]");
      if (requestBot && nextBot && !requestBot.querySelector("[data-seerr-search-form]")) {
        dashboard.replaceNodeContent(requestBot, nextBot);
      }
      var token = doc.querySelector('input[name="csrf_token"]');
      if (token) {
        dashboard.queryAll('input[name="csrf_token"]').forEach(function (input) {
          input.value = token.value;
        });
      }
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
