(function () {
  const menus = Array.from(document.querySelectorAll("[data-mobile-nav-details]"));
  if (menus.length === 0) {
    return;
  }

  function closeMenu(menu, focusSummary) {
    if (!menu.open) {
      return;
    }
    menu.open = false;
    if (focusSummary) {
      const summary = menu.querySelector("summary");
      if (summary) {
        summary.focus();
      }
    }
  }

  document.addEventListener("pointerdown", function (event) {
    for (const menu of menus) {
      if (menu.open && !menu.contains(event.target)) {
        closeMenu(menu, false);
      }
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.key !== "Escape") {
      return;
    }
    for (const menu of menus) {
      closeMenu(menu, true);
    }
  });
})();
