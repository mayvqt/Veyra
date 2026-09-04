(function (dashboard) {
  function inferKind(row) {
    var kind = (row.getAttribute("data-kind") || "").toLowerCase();
    if (kind === "movie" || kind === "tv") {
      return kind;
    }
    var source = (row.getAttribute("data-source") || "").toLowerCase();
    if (source === "radarr") {
      return "movie";
    }
    if (source === "sonarr") {
      return "tv";
    }
    return "";
  }

  function initQueueFilter() {
    var list = document.getElementById("download-queue-list");
    if (!list) {
      return;
    }
    var buttons = dashboard.queryAll(".queue-filter-btn");
    if (buttons.length === 0) {
      return;
    }
    var panel = list.closest("[data-dashboard-refresh='queue']") || list.parentElement;
    function applyFilter(filter) {
      var normalized = String(filter || "all").toLowerCase();
      if (panel) {
        panel.setAttribute("data-queue-filter", normalized);
      }
      var rows = dashboard.queryAll(".queue-row", list);
      var visibleRows = 0;
      rows.forEach(function (row) {
        var kind = inferKind(row);
        var visible = normalized === "all" || kind === normalized;
        row.style.display = visible ? "" : "none";
        if (visible) {
          visibleRows += 1;
        }
      });
      var emptyMessage = list.parentElement.querySelector(".queue-filter-empty");
      if (emptyMessage) {
        emptyMessage.hidden = visibleRows > 0;
      }
      buttons.forEach(function (btn) {
        var isActive = btn.getAttribute("data-kind-filter") === normalized;
        btn.classList.toggle("is-active", isActive);
        btn.setAttribute("aria-pressed", String(isActive));
      });
    }
    buttons.forEach(function (btn) {
      btn.addEventListener("click", function () {
        applyFilter(btn.getAttribute("data-kind-filter") || "all");
      });
    });
    var savedFilter = panel && panel.getAttribute("data-queue-filter");
    var activeButton = null;
    buttons.some(function (btn) {
      if (btn.classList.contains("is-active")) {
        activeButton = btn;
        return true;
      }
      return false;
    });
    applyFilter(savedFilter || (activeButton && activeButton.getAttribute("data-kind-filter")) || "all");
  }

  dashboard.registerWidget(initQueueFilter);
})(window.VeyraDashboard);
