(function (dashboard) {
  function localDateKey(date) {
    var year = date.getFullYear();
    var month = String(date.getMonth() + 1).padStart(2, "0");
    var day = String(date.getDate()).padStart(2, "0");
    return year + "-" + month + "-" + day;
  }

  function initCalendarLocalTimes() {
    var nodes = dashboard.queryAll(".calendar-local-time[data-utc]");
    if (nodes.length === 0) {
      return;
    }
    nodes.forEach(function (node) {
      var raw = node.getAttribute("data-utc");
      if (!raw) {
        return;
      }
      var dt = new Date(raw);
      if (isNaN(dt.getTime())) {
        return;
      }
      var time = dt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
      var zone = dt.toLocaleTimeString([], { timeZoneName: "short" }).split(" ").pop();
      node.textContent = zone ? (time + " " + zone) : time;
    });
  }

  function initCalendarToday() {
    var today = localDateKey(new Date());
    var days = dashboard.queryAll(".calendar-day-row[data-date]");
    days.forEach(function (day) {
      day.classList.toggle("is-today", day.getAttribute("data-date") === today);
    });
  }

  dashboard.registerWidget(initCalendarLocalTimes);
  dashboard.registerWidget(initCalendarToday);
})(window.VeyraDashboard);
