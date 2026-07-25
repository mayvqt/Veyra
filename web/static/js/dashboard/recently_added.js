(function (dashboard) {
  function initRecentlyAddedImageFallback() {
    var list = document.getElementById("recently-added-list");
    if (!list) {
      return;
    }
    var posters = dashboard.queryAll("img.poster", list);
    posters.forEach(function (img) {
      img.addEventListener("error", function () {
        var card = img.closest(".media-card");
        if (!card) {
          return;
        }
        img.remove();
        var fallback = document.createElement("div");
        fallback.className = "poster poster-empty";
        fallback.textContent = "Poster unavailable";
        card.insertBefore(fallback, card.firstChild);
      });
    });
  }

  dashboard.registerWidget(initRecentlyAddedImageFallback);
})(window.VeyraDashboard);
