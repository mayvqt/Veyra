(function (dashboard) {
  function mediaKindLabel(mediaType) {
    return mediaType === "tv" ? "Show" : "Movie";
  }

  function posterURL(path) {
    if (!path) {
      return "";
    }
    if (/^https?:\/\//i.test(path)) {
      return path;
    }
    if (/^\/\//.test(path)) {
      return "https:" + path;
    }
    if (path.charAt(0) === "/") {
      return "https://image.tmdb.org/t/p/w342" + path;
    }
    return "";
  }

  function parseJSONResponse(response, fallbackMessage) {
    var contentType = response.headers ? (response.headers.get("Content-Type") || "") : "";
    if (contentType.indexOf("application/json") === -1) {
      if (response.status === 401 || response.redirected) {
        throw new Error("Your session expired. Log in again.");
      }
      throw new Error(fallbackMessage);
    }
    return response.json().then(function (data) {
      if (!response.ok) {
        throw new Error(data.error || fallbackMessage);
      }
      return data;
    }, function () {
      throw new Error(fallbackMessage);
    });
  }

  function renderSearchResult(row, csrfToken) {
    var item = document.createElement("li");
    item.className = "request-bot-result";

    var poster = posterURL(row.posterUrl || row.posterPath);
    if (poster) {
      item.classList.add("has-poster");
      var image = document.createElement("img");
      image.className = "request-bot-poster";
      image.src = poster;
      image.alt = (row.title || "Media") + " poster";
      image.loading = "lazy";
      image.decoding = "async";
      image.addEventListener("error", function () {
        item.classList.remove("has-poster");
        image.remove();
      });
      item.appendChild(image);
    }

    var main = document.createElement("div");
    main.className = "request-bot-result-main";

    var title = document.createElement("strong");
    title.textContent = row.title || "Untitled";
    main.appendChild(title);

    var meta = document.createElement("span");
    meta.className = "meta-line";
    var pieces = [mediaKindLabel(row.mediaType)];
    if (row.year) {
      pieces.push(row.year);
    }
    if (row.status) {
      pieces.push(row.status);
    }
    meta.textContent = pieces.join(" · ");
    main.appendChild(meta);

    if (row.overview) {
      var overview = document.createElement("span");
      overview.className = "request-bot-overview";
      overview.textContent = row.overview;
      main.appendChild(overview);
    }

    if (row.mediaType === "tv" && row.canRequest && row.seasons && row.seasons.length > 0) {
      var picker = document.createElement("fieldset");
      picker.className = "request-bot-season";
      var pickerText = document.createElement("legend");
      pickerText.textContent = "Seasons";
      var choices = document.createElement("div");
      choices.className = "request-bot-season-options";
      row.seasons.forEach(function (season) {
        var choice = document.createElement("label");
        choice.className = "request-bot-season-choice";
        var checkbox = document.createElement("input");
        checkbox.type = "checkbox";
        checkbox.value = String(season.number || "");
        checkbox.setAttribute("data-season-checkbox", "");
        var number = document.createElement("span");
        number.textContent = String(season.number || "");
        choice.title = season.name || ("Season " + season.number);
        if (season.episodeCount) {
          choice.title += " · " + season.episodeCount + " eps";
        }
        choice.appendChild(checkbox);
        choice.appendChild(number);
        choices.appendChild(choice);
      });
      picker.appendChild(pickerText);
      picker.appendChild(choices);
      main.appendChild(picker);
    }

    var actions = document.createElement("div");
    actions.className = "request-bot-actions";

    if (row.openUrl) {
      var open = document.createElement("a");
      open.className = "btn secondary";
      open.href = row.openUrl;
      open.textContent = "Open";
      actions.appendChild(open);
    }

    var request = document.createElement("button");
    request.className = "btn";
    request.type = "button";
    request.textContent = row.canRequest ? "Request" : (row.status || "Unavailable");
    request.disabled = !row.canRequest;
    request.setAttribute("data-media-id", String(row.id || ""));
    request.setAttribute("data-media-type", row.mediaType || "");
    request.setAttribute("data-csrf", csrfToken || "");
    actions.appendChild(request);

    item.appendChild(main);
    item.appendChild(actions);
    return item;
  }

  function initSeerrRequestBot() {
    var root = document.querySelector("[data-seerr-request-bot]");
    if (!root || root.getAttribute("data-enhanced") === "true") {
      return;
    }
    root.setAttribute("data-enhanced", "true");
    var form = root.querySelector("[data-seerr-search-form]");
    var query = root.querySelector("[data-seerr-query]");
    var csrf = root.querySelector("[data-seerr-csrf]");
    var results = root.querySelector("[data-seerr-results]");
    var message = root.querySelector("[data-seerr-message]");
    if (!form || !query || !results) {
      return;
    }

    var searchSequence = 0;
    var searchController = null;
    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var requestSequence = ++searchSequence;
      if (searchController && typeof searchController.abort === "function") {
        searchController.abort();
      }
      searchController = typeof window.AbortController === "function" ? new window.AbortController() : null;
      var q = (query.value || "").trim();
      dashboard.clearNode(results);
      if (q.length < 2) {
        dashboard.setMessage(message, "Type at least 2 characters.", "error");
        return;
      }
      dashboard.setMessage(message, "Searching...", "");
      var fetchOptions = {
        headers: { "Accept": "application/json" },
        credentials: "same-origin",
        cache: "no-store"
      };
      if (searchController) {
        fetchOptions.signal = searchController.signal;
      }
      fetch("/api/seerr/search?q=" + encodeURIComponent(q), fetchOptions).then(function (response) {
        return parseJSONResponse(response, "Search failed.");
      }).then(function (data) {
        if (requestSequence !== searchSequence) {
          return;
        }
        var rows = data.results || [];
        dashboard.clearNode(results);
        if (rows.length === 0) {
          dashboard.setMessage(message, "No matches found.", "");
          return;
        }
        dashboard.setMessage(message, "", "");
        rows.forEach(function (row) {
          results.appendChild(renderSearchResult(row, csrf ? csrf.value : ""));
        });
      }).catch(function (err) {
        if (requestSequence !== searchSequence || err.name === "AbortError") {
          return;
        }
        dashboard.setMessage(message, err.message || "Search failed.", "error");
      });
    });

    results.addEventListener("click", function (event) {
      var btn = event.target.closest("button[data-media-id]");
      if (!btn || btn.disabled) {
        return;
      }
      var body = new URLSearchParams();
      var row = btn.closest(".request-bot-result");
      var seasonChecks = row ? dashboard.queryAll("[data-season-checkbox]:checked", row) : [];
      if (btn.getAttribute("data-media-type") === "tv" && seasonChecks.length === 0) {
        dashboard.setMessage(message, "Choose a season before requesting.", "error");
        return;
      }
      body.set("csrf_token", btn.getAttribute("data-csrf") || "");
      body.set("media_id", btn.getAttribute("data-media-id") || "");
      body.set("media_type", btn.getAttribute("data-media-type") || "");
      if (seasonChecks.length > 0) {
        body.set("seasons", seasonChecks.map(function (checkbox) {
          return checkbox.value;
        }).filter(Boolean).join(","));
      }
      btn.disabled = true;
      btn.textContent = "Sending";
      dashboard.setMessage(message, "", "");
      fetch("/api/seerr/request", {
        method: "POST",
        headers: { "Accept": "application/json", "Content-Type": "application/x-www-form-urlencoded" },
        credentials: "same-origin",
        body: body.toString()
      }).then(function (response) {
        return parseJSONResponse(response, "Request failed.");
      }).then(function (data) {
        btn.textContent = data.status || "Requested";
        dashboard.setMessage(message, data.message || "Request sent to Seerr.", "success");
      }).catch(function (err) {
        btn.disabled = false;
        btn.textContent = "Request";
        dashboard.setMessage(message, err.message || "Request failed.", "error");
      });
    });
  }

  dashboard.registerWidget(initSeerrRequestBot);
})(window.VeyraDashboard);
