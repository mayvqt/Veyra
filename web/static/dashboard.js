(function () {
  var scripts = [
    "/static/js/dashboard/namespace.js",
    "/static/js/dashboard/request_bot.js",
    "/static/js/dashboard/queue_filter.js",
    "/static/js/dashboard/recently_added.js",
    "/static/js/dashboard/calendar.js",
    "/static/js/dashboard/refresh.js"
  ];

  function staticVersion() {
    var current = document.currentScript;
    if (!current || !current.src) {
      return "";
    }
    try {
      return new URL(current.src, window.location.href).search;
    } catch (err) {
      return "";
    }
  }

  var versionQuery = staticVersion();

  function loadNext(index) {
    if (index >= scripts.length) {
      if (window.VeyraDashboard) {
        window.VeyraDashboard.initWidgets();
        window.VeyraDashboard.initAutoRefresh();
      }
      return;
    }

    var script = document.createElement("script");
    script.src = scripts[index] + versionQuery;
    script.onload = function () {
      loadNext(index + 1);
    };
    script.onerror = function () {
      if (window.console && typeof window.console.error === "function") {
        window.console.error("Failed to load dashboard script:", scripts[index]);
      }
      loadNext(index + 1);
    };
    document.head.appendChild(script);
  }

  loadNext(0);
})();
