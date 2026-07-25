(function (window, document) {
  var dashboard = window.VeyraDashboard || {};
  var widgets = [];

  dashboard.setMessage = function (node, text, kind) {
    if (!node) {
      return;
    }
    node.textContent = text || "";
    node.classList.toggle("is-error", kind === "error");
    node.classList.toggle("is-success", kind === "success");
    node.hidden = !text;
  };

  dashboard.clearNode = function (node) {
    if (node && typeof node.replaceChildren === "function") {
      node.replaceChildren();
    } else if (node) {
      node.textContent = "";
    }
  };

  dashboard.queryAll = function (selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  };

  function importChildren(source) {
    return Array.prototype.map.call(source.childNodes, function (child) {
      return document.importNode(child, true);
    });
  }

  dashboard.replaceNodeContent = function (target, source) {
    if (!target || !source) {
      return;
    }
    var children = importChildren(source);
    if (typeof target.replaceChildren === "function") {
      target.replaceChildren.apply(target, children);
      return;
    }
    target.textContent = "";
    children.forEach(function (child) {
      target.appendChild(child);
    });
  };

  dashboard.registerWidget = function (init) {
    if (typeof init === "function") {
      widgets.push(init);
    }
  };

  dashboard.initWidgets = function () {
    widgets.forEach(function (init) {
      try {
        init();
      } catch (err) {
        if (window.console && typeof window.console.error === "function") {
          window.console.error("Dashboard widget failed to initialize:", err);
        }
      }
    });
  };

  dashboard.initAutoRefresh = dashboard.initAutoRefresh || function () {};

  window.VeyraDashboard = dashboard;
})(window, document);
