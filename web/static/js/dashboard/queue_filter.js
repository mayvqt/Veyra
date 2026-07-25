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
    function applyFilter(filter) {
      var normalized = String(filter || "all").toLowerCase();
      var rows = dashboard.queryAll(".queue-row", list);
      rows.forEach(function (row) {
        var kind = inferKind(row);
        row.style.display = normalized === "all" || kind === normalized ? "" : "none";
      });
      buttons.forEach(function (btn) {
        btn.classList.toggle("is-active", btn.getAttribute("data-kind-filter") === normalized);
      });
    }
    buttons.forEach(function (btn) {
      btn.addEventListener("click", function () {
        applyFilter(btn.getAttribute("data-kind-filter") || "all");
      });
    });
    applyFilter("all");
  }

  dashboard.registerWidget(initQueueFilter);
})(window.VeyraDashboard);
