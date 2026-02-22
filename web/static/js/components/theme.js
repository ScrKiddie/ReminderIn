import { t } from "../i18n/lang.js";

export function initThemeToggle() {
  const themeToggle = document.getElementById("theme-toggle");

  const updateToggleText = (theme) => {
    themeToggle.textContent = theme === "dark" ? t("lightMode") : t("darkMode");
  };

  updateToggleText(
    document.documentElement.getAttribute("data-theme") || "light",
  );

  themeToggle.addEventListener("click", () => {
    let currentTheme = document.documentElement.getAttribute("data-theme");
    let targetTheme = currentTheme === "light" ? "dark" : "light";

    document.documentElement.setAttribute("data-theme", targetTheme);
    localStorage.setItem("rm_theme", targetTheme);
    updateToggleText(targetTheme);
  });
}
