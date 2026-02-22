import { t, currentLang, applyLanguage } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { initWA } from "./wa-connection.js";
import { loadReminders } from "./reminders-table.js";

const loginView = document.getElementById("login-view");
const appView = document.getElementById("app-view");
const logoutBtn = document.getElementById("logout-btn");
const loginForm = document.getElementById("login-form");

export async function showLogin() {
  loginView.hidden = false;
  appView.hidden = true;
  logoutBtn.hidden = true;
}

export async function showApp() {
  loginView.hidden = true;
  appView.hidden = false;
  logoutBtn.hidden = false;
  await Promise.all([initWA(), loadReminders()]);
}

export function initAuth() {
  document
    .getElementById("toggle-password")
    .addEventListener("click", function () {
      const pwd = document.getElementById("login-password");
      const isHidden = pwd.type === "password";
      pwd.type = isHidden ? "text" : "password";
      document.getElementById("eye-open").style.display = isHidden
        ? "none"
        : "";
      document.getElementById("eye-closed").style.display = isHidden
        ? ""
        : "none";
      this.title = isHidden ? "Sembunyikan password" : "Tampilkan password";
    });

  loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const username = document.getElementById("login-username").value;
    const password = document.getElementById("login-password").value;
    const rememberMe = document.getElementById("login-remember").checked;

    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password, rememberMe }),
      });

      if (res.ok) {
        showMsg(t("loginSuccess"));
        showApp();
      } else if (res.status === 429) {
        const retryAfterSec = await getRetryAfterSeconds(res);
        showMsg(rateLimitMessage(retryAfterSec), true, 5000);
      } else {
        showMsg(t("loginFailed"), true);
      }
    } catch (err) {
      showMsg(t("loginError"), true);
    }
  });

  logoutBtn.addEventListener("click", async () => {
    loginForm.reset();
    try {
      await fetch("/api/logout", { method: "POST" });
    } catch (e) {}
    showMsg(t("logoutSuccess"));
    showLogin();
  });
}

async function getRetryAfterSeconds(res) {
  let seconds = parsePositiveInt(res.headers.get("Retry-After"));
  try {
    const contentType = res.headers.get("Content-Type") || "";
    if (contentType.includes("application/json")) {
      const data = await res.json();
      const fromBody = parsePositiveInt(data?.retry_after_seconds);
      if (fromBody > 0) {
        seconds = Math.max(seconds, fromBody);
      }
    }
  } catch (e) {}
  return seconds;
}

function parsePositiveInt(value) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return parsed;
}

function rateLimitMessage(retryAfterSec) {
  if (!retryAfterSec) {
    return t("loginRateLimitedUnknown");
  }
  const minute = Math.floor(retryAfterSec / 60);
  const second = retryAfterSec % 60;
  if (minute === 0) {
    return `${t("loginRateLimited")} ${second} ${t("timeSeconds")}.`;
  }
  if (second === 0) {
    return `${t("loginRateLimited")} ${minute} ${t("timeMinutes")}.`;
  }
  return `${t("loginRateLimited")} ${minute} ${t("timeMinutes")} ${second} ${t("timeSeconds")}.`;
}

export async function bootApp() {
  applyLanguage(currentLang);
  try {
    const res = await fetch("/api/session");
    if (res.ok) {
      await showApp();
    } else {
      await showLogin();
    }
  } catch (e) {
    await showLogin();
  } finally {
    document.getElementById("app-container").style.opacity = "1";
  }
}
