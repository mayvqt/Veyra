(function (dashboard) {
  var refreshIntervalMs = 45000;
  var returnRefreshDelayMs = 250;

  function dashboardIsSafeToRefresh() {
    if (document.hidden) {
      return false;
    }
    var requestQuery = document.querySelector("[data-seerr-query]");
    if (requestQuery && requestQuery.value.trim() !== "") {
      return false;
    }
    var active = document.activeElement;
    if (!active) {
      return true;
    }
    return !active.matches("input, textarea, select, button");
  }

  function refreshDashboardContent() {
    if (!dashboardIsSafeToRefresh()) {
      return;
    }
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
      var next = doc.querySelector("[data-dashboard-container]");
      var current = document.querySelector("[data-dashboard-container]");
      if (!next || !current) {
        return;
      }
      dashboard.replaceNodeContent(current, next);
      dashboard.initWidgets();
    }).catch(function () {});
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
