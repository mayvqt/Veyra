(function (dashboard) {
  // Server week boundaries and day groups use UTC; keep the clock and today's
  // highlight in that same timezone.
  dashboard.registerWidget(function () {
    var today = new Date().toISOString().slice(0, 10);
    dashboard.queryAll(".calendar-day-row[data-date]").forEach(function (day) {
      day.classList.toggle("is-today", day.getAttribute("data-date") === today);
    });
  });
})(window.VeyraDashboard);
