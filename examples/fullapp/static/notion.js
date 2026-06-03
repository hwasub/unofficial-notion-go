(function () {
  "use strict";

  document.addEventListener("click", async function (event) {
    var target = event.target;
    if (!(target instanceof Element)) return;

    var copyCodeButton = target.closest("[data-copy-code]");
    if (copyCodeButton) {
      var codeBlock = copyCodeButton.closest(".notion-code");
      var value = codeBlock && codeBlock.querySelector("pre")
        ? codeBlock.querySelector("pre").textContent || ""
        : "";
      if (value) await copyWithButtonFeedback(copyCodeButton, value);
      return;
    }

    var lightboxTrigger = target.closest("[data-collection-lightbox]");
    if (lightboxTrigger) {
      event.preventDefault();
      openCollectionLightbox(lightboxTrigger);
      return;
    }

    var tabButton = target.closest("[data-notion-tab-target]");
    if (tabButton) {
      event.preventDefault();
      activateNotionTab(tabButton);
      return;
    }

    var lightboxAction = target.closest("[data-lightbox-action]");
    if (lightboxAction) {
      var action = lightboxAction.getAttribute("data-lightbox-action");
      if (action === "close") closeCollectionLightbox();
      if (action === "previous") stepCollectionLightbox(-1);
      if (action === "next") stepCollectionLightbox(1);
      return;
    }

    var lightboxRoot = target.closest(".collection-lightbox");
    if (
      lightboxRoot &&
      (target === lightboxRoot || target.classList.contains("collection-lightbox__stage"))
    ) {
      closeCollectionLightbox();
    }
  });

  document.addEventListener(
    "error",
    function (event) {
      if (event.target instanceof HTMLImageElement) {
        hideFailedPreviewImage(event.target);
      }
    },
    true,
  );

  document.addEventListener("DOMContentLoaded", function () {
    renderNotionEquations();
    hideAlreadyFailedPreviewImages();
  });

  document.addEventListener("keydown", function (event) {
    var target = event.target;
    var tabButton = target instanceof Element
      ? target.closest("[data-notion-tab-target]")
      : null;
    if (
      tabButton &&
      (event.key === "ArrowLeft" ||
        event.key === "ArrowRight" ||
        event.key === "Home" ||
        event.key === "End")
    ) {
      event.preventDefault();
      stepNotionTab(tabButton, event.key);
      return;
    }

    var lightbox = document.querySelector(".collection-lightbox:not([hidden])");
    if (!lightbox) return;

    if (event.key === "Escape") {
      event.preventDefault();
      closeCollectionLightbox();
      return;
    }
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      stepCollectionLightbox(-1);
      return;
    }
    if (event.key === "ArrowRight") {
      event.preventDefault();
      stepCollectionLightbox(1);
      return;
    }
    if (event.key === "Tab") {
      trapLightboxFocus(event, lightbox);
    }
  });

  async function copyWithButtonFeedback(button, value) {
    try {
      await navigator.clipboard.writeText(value);
      var original = button.textContent;
      button.textContent = button.getAttribute("data-copy-success") || original;
      button.setAttribute("data-copied", "");
      window.setTimeout(function () {
        button.textContent = original;
        button.removeAttribute("data-copied");
      }, 1200);
    } catch {
      window.prompt(button.getAttribute("data-copy-prompt") || "", value);
    }
  }

  var collectionLightboxIndex = -1;
  var collectionLightboxTrigger = null;

  function collectionLightboxButtons() {
    return Array.from(document.querySelectorAll("[data-collection-lightbox]"));
  }

  function collectionLightboxLabels() {
    var source = document.querySelector("[data-collection-lightbox-labels]");
    var read = function (name, fallback) {
      return (source && source.getAttribute(name)) || fallback;
    };
    return {
      previous: read("data-lightbox-previous", "Previous"),
      next: read("data-lightbox-next", "Next"),
      close: read("data-lightbox-close", "Close"),
      dialog: read("data-lightbox-label", "Image viewer"),
    };
  }

  function ensureCollectionLightbox() {
    var lightbox = document.querySelector(".collection-lightbox");
    if (lightbox) return lightbox;

    var labels = collectionLightboxLabels();
    lightbox = document.createElement("div");
    lightbox.className = "collection-lightbox";
    lightbox.hidden = true;
    lightbox.setAttribute("role", "dialog");
    lightbox.setAttribute("aria-modal", "true");
    lightbox.setAttribute("aria-label", labels.dialog);

    var bar = document.createElement("div");
    bar.className = "collection-lightbox__bar";
    bar.append(
      lightboxButton("previous", labels.previous),
      lightboxButton("next", labels.next),
      lightboxButton("close", labels.close),
    );

    var stage = document.createElement("div");
    stage.className = "collection-lightbox__stage";
    var image = document.createElement("img");
    image.alt = "";
    stage.appendChild(image);

    lightbox.append(bar, stage);
    document.body.appendChild(lightbox);
    return lightbox;
  }

  function lightboxButton(action, text) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "collection-lightbox__button collection-lightbox__button--" + action;
    button.setAttribute("data-lightbox-action", action);
    button.textContent = text;
    return button;
  }

  function openCollectionLightbox(trigger) {
    var buttons = collectionLightboxButtons();
    collectionLightboxIndex = Math.max(0, buttons.indexOf(trigger));
    collectionLightboxTrigger = trigger;
    renderCollectionLightbox();
  }

  function renderCollectionLightbox() {
    var buttons = collectionLightboxButtons();
    var trigger = buttons[collectionLightboxIndex];
    if (!trigger) return;

    var source = trigger.getAttribute("data-full-src") || trigger.getAttribute("href");
    if (!source) return;

    var lightbox = ensureCollectionLightbox();
    var image = lightbox.querySelector("img");
    var alt =
      trigger.getAttribute("data-alt") ||
      (trigger.querySelector("img") && trigger.querySelector("img").getAttribute("alt")) ||
      "";
    image.setAttribute("src", source);
    image.setAttribute("alt", alt);
    lightbox.hidden = false;
    document.body.style.overflow = "hidden";

    var main = document.querySelector(".notion-body-wrap");
    if (main) {
      main.inert = true;
      main.setAttribute("aria-hidden", "true");
    }
    var closeButton = lightbox.querySelector("[data-lightbox-action='close']");
    if (closeButton) closeButton.focus();
  }

  function closeCollectionLightbox() {
    var lightbox = document.querySelector(".collection-lightbox");
    if (!lightbox) return;

    lightbox.hidden = true;
    var image = lightbox.querySelector("img");
    if (image) image.removeAttribute("src");
    document.body.style.overflow = "";

    var main = document.querySelector(".notion-body-wrap");
    if (main) {
      main.inert = false;
      main.removeAttribute("aria-hidden");
    }
    if (collectionLightboxTrigger && collectionLightboxTrigger.focus) {
      collectionLightboxTrigger.focus();
    }
    collectionLightboxTrigger = null;
    collectionLightboxIndex = -1;
  }

  function stepCollectionLightbox(delta) {
    var buttons = collectionLightboxButtons();
    if (buttons.length === 0 || collectionLightboxIndex < 0) return;
    collectionLightboxIndex = (collectionLightboxIndex + delta + buttons.length) % buttons.length;
    collectionLightboxTrigger = buttons[collectionLightboxIndex];
    renderCollectionLightbox();
  }

  function trapLightboxFocus(event, lightbox) {
    var focusables = Array.from(lightbox.querySelectorAll("[data-lightbox-action]"));
    if (focusables.length === 0) return;
    var first = focusables[0];
    var last = focusables[focusables.length - 1];
    var active = document.activeElement;
    if (!lightbox.contains(active)) {
      event.preventDefault();
      first.focus();
    } else if (event.shiftKey && active === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function activateNotionTab(button) {
    var root = button.closest("[data-notion-tabs]");
    if (!root) return;
    var targetID = button.getAttribute("data-notion-tab-target");
    if (!targetID) return;

    notionTabButtons(root).forEach(function (item) {
      var selected = item === button;
      item.classList.toggle("notion-tab-button--active", selected);
      item.setAttribute("aria-selected", selected ? "true" : "false");
      item.setAttribute("tabindex", selected ? "0" : "-1");
    });
    notionTabPanels(root).forEach(function (panel) {
      panel.hidden = panel.id !== targetID;
    });
  }

  function stepNotionTab(button, key) {
    var root = button.closest("[data-notion-tabs]");
    if (!root) return;
    var buttons = notionTabButtons(root);
    var current = buttons.indexOf(button);
    if (current < 0) return;

    var next = current;
    if (key === "ArrowLeft") next = current - 1;
    if (key === "ArrowRight") next = current + 1;
    if (key === "Home") next = 0;
    if (key === "End") next = buttons.length - 1;
    next = (next + buttons.length) % buttons.length;

    var nextButton = buttons[next];
    activateNotionTab(nextButton);
    nextButton.focus();
  }

  function notionTabButtons(root) {
    return Array.from(root.querySelectorAll("[data-notion-tab-target]")).filter(function (item) {
      return item.closest("[data-notion-tabs]") === root;
    });
  }

  function notionTabPanels(root) {
    return Array.from(root.querySelectorAll("[data-notion-tab-panel]")).filter(function (item) {
      return item.closest("[data-notion-tabs]") === root;
    });
  }

  function renderNotionEquations() {
    var katex = window.katex;
    if (!katex || typeof katex.render !== "function") return;
    document.querySelectorAll(".notion-equation").forEach(function (node) {
      if (node.querySelector(".katex")) return;
      var tex = (node.getAttribute("data-tex") || node.textContent || "").trim();
      if (!tex) return;
      try {
        katex.render(tex, node, {
          displayMode: !node.classList.contains("notion-equation--inline"),
          throwOnError: false,
          strict: "ignore",
          trust: false,
        });
      } catch {
        // Keep the escaped TeX fallback already present in the HTML.
      }
    });
  }

  function hideAlreadyFailedPreviewImages() {
    document.querySelectorAll("img").forEach(function (image) {
      if (image.complete && image.naturalWidth === 0) {
        hideFailedPreviewImage(image);
      }
    });
  }

  function hideFailedPreviewImage(image) {
    if (
      !image.matches(
        ".notion-bookmark__icon, .notion-bookmark__cover img, .notion-google-drive__icon, .notion-google-drive__preview img",
      )
    ) {
      return;
    }
    var removableFrame = image.closest(".notion-bookmark__cover, .notion-google-drive__preview");
    if (removableFrame) {
      removableFrame.hidden = true;
      removableFrame.setAttribute("aria-hidden", "true");
      return;
    }
    image.hidden = true;
    image.setAttribute("aria-hidden", "true");
  }
})();
