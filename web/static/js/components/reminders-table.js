import { t } from "../i18n/lang.js";
import { escapeHtml, formatWhatsAppMarkdown } from "../utils/html.js";
import { cronToText, formatHumanDate } from "../utils/format.js";
import { fetchRemindersApi } from "../api/reminders.js";
import { state, globals } from "../store/state.js";

const remindersList = document.getElementById("reminders-list");
const prevBtn = document.getElementById("prev-btn");
const nextBtn = document.getElementById("next-btn");
const pageInfo = document.getElementById("page-info");
const pageSizeSelect = document.getElementById("page-size");
const searchInput = document.getElementById("search-input");

export async function loadReminders(fresh = true) {
  if (globals.activeController) {
    globals.activeController.abort();
  }
  globals.activeController = new AbortController();
  const signal = globals.activeController.signal;

  try {
    if (fresh) {
      state.currentCursor = null;
      state.cursorStack = [];
      if (remindersList) remindersList.closest("table").style.opacity = "0.5";
    }

    const res = await fetchRemindersApi(
      {
        limit: state.pageSize,
        cursor: state.currentCursor,
        search: state.searchTerm,
        sortBy: state.sortBy,
        sortOrder: state.sortOrder,
      },
      signal,
      state.lastETag,
    );

    if (res.status === 304) {
      if (remindersList) remindersList.closest("table").style.opacity = "1";
      return;
    }

    if (!res.ok) return;

    const jsonRes = await res.json();
    const data = jsonRes.data;

    const total = jsonRes.meta ? jsonRes.meta.total : jsonRes.total;
    const next_cursor = jsonRes.meta
      ? jsonRes.meta.next_cursor
      : jsonRes.next_cursor;

    const etag = res.headers.get("ETag");
    if (etag) state.lastETag = etag;

    state.totalReminders = total || 0;
    state.tempNextCursor = next_cursor || null;

    renderTable(data || []);
    updatePaginationUI();
    updateSortIndicators();
  } catch (err) {
    if (err.name !== "AbortError") {
      console.error("Gagal memuat pengingat", err);
    }
  } finally {
    if (remindersList) remindersList.closest("table").style.opacity = "1";
  }
}

function buildRowCellsHTML(rem) {
  const recSentence = cronToText(rem.recurrence);
  const isRecurring = rem.recurrence && rem.recurrence.trim() !== "";
  const isExpired = !isRecurring && new Date(rem.scheduled_at) < new Date();
  const toggleDisabled = isExpired;

  return `
    <td data-label="${t("thMessage")}">
        <div style="max-height: 120px; overflow-y: auto; overflow-wrap: break-word; font-size: 0.95em; white-space: pre-wrap;">${formatWhatsAppMarkdown(rem.message)}</div>
    </td>
    <td data-label="${t("thTarget")}">
        <div style="max-height: 120px; overflow-y: auto; overflow-wrap: break-word; font-size: 0.95em; white-space: pre-wrap;">${escapeHtml(rem.target_wa ? rem.target_wa.split(",").join(", ") : t("yourself"))}</div>
    </td>
    <td data-label="${t("thNextTime")}">
        <div style="font-weight: 500;">${escapeHtml(formatHumanDate(rem.scheduled_at))}</div>
    </td>
    <td data-label="${t("thRecurrence")}">
        <div style="font-weight: 500;">${escapeHtml(recSentence)}</div>
    </td>
    <td data-label="${t("thStatus")}" align="center">
        <input type="checkbox" onchange="toggleReminder('${rem.id}')" 
            ${rem.is_active ? "checked" : ""} 
            ${toggleDisabled ? "disabled" : ""}>
    </td>
    <td data-label="${t("thActions")}" style="white-space: nowrap;">
        <div class="action-buttons" style="display: flex; gap: 8px; justify-content: center;">
            <button onclick="editReminder('${rem.id}')">${t("edit")}</button>
            <button onclick="deleteReminder('${rem.id}')">${t("delete")}</button>
        </div>
    </td>`;
}

function renderTable(reminders) {
  if (!remindersList) return;
  if (reminders.length === 0) {
    remindersList.innerHTML = `<tr class="empty-row"><td class="empty-cell" colspan="6" align="center">${t("noReminders")}</td></tr>`;
    return;
  }

  const existingRows = {};
  for (const row of remindersList.querySelectorAll("tr[data-id]")) {
    existingRows[row.getAttribute("data-id")] = row;
  }

  const emptyRow = remindersList.querySelector("tr:not([data-id])");
  if (emptyRow) emptyRow.remove();

  const newIds = new Set(reminders.map((r) => r.id));

  for (const [id, row] of Object.entries(existingRows)) {
    if (!newIds.has(id)) {
      row.remove();
      delete existingRows[id];
    }
  }

  const fragment = document.createDocumentFragment();
  const orderedRows = [];

  for (const rem of reminders) {
    const newHTML = buildRowCellsHTML(rem);
    let row = existingRows[rem.id];

    if (row) {
      if (row.innerHTML.trim() !== newHTML.trim()) {
        row.innerHTML = newHTML;
      }
      orderedRows.push(row);
    } else {
      row = document.createElement("tr");
      row.setAttribute("data-id", rem.id);
      row.innerHTML = newHTML;
      orderedRows.push(row);
    }
  }

  for (const row of orderedRows) {
    fragment.appendChild(row);
  }
  remindersList.innerHTML = "";
  remindersList.appendChild(fragment);

  state.remindersData = reminders;
}

export function rerenderRemindersLocale() {
  if (!remindersList) return;
  renderTable(state.remindersData || []);
  updatePaginationUI();
  updateSortIndicators();
}

function updatePaginationUI() {
  const start = state.cursorStack.length * state.pageSize + 1;
  const countOnPage = remindersList
    ? remindersList.querySelectorAll("tr[data-id]").length
    : 0;
  const end = start + countOnPage - 1;
  const total = state.totalReminders;

  if (pageInfo) {
    if (total === 0) {
      pageInfo.textContent = `${t("showing")} 0-0 ${t("of")} 0`;
    } else {
      pageInfo.textContent = `${t("showing")} ${start}-${end} ${t("of")} ${total}`;
    }
  }

  if (prevBtn) prevBtn.disabled = state.cursorStack.length === 0;
  if (nextBtn) nextBtn.disabled = !state.tempNextCursor;
}

function updateSortIndicators() {
  ["message", "target", "time", "recurrence"].forEach((col) => {
    const span = document.getElementById(`sort-${col}`);
    if (!span) return;
    if (state.sortBy === col) {
      span.textContent = state.sortOrder === "asc" ? " ↑" : " ↓";
    } else {
      span.textContent = "";
    }
  });

  const mobileSort = document.getElementById("mobile-sort");
  if (mobileSort) {
    mobileSort.value = state.sortBy || "time";
  }
}

export function initTableInteractions() {
  window.setSort = (col) => {
    if (state.sortBy === col) {
      state.sortOrder = state.sortOrder === "asc" ? "desc" : "asc";
    } else {
      state.sortBy = col;
      state.sortOrder = "asc";
    }
    state.lastETag = null;
    loadReminders(true);
  };

  if (nextBtn)
    nextBtn.addEventListener("click", () => {
      state.cursorStack.push(state.currentCursor);
      state.currentCursor = state.tempNextCursor;
      state.lastETag = null;
      loadReminders(false);
    });

  if (prevBtn)
    prevBtn.addEventListener("click", () => {
      state.currentCursor = state.cursorStack.pop();
      state.lastETag = null;
      loadReminders(false);
    });

  const mobileSort = document.getElementById("mobile-sort");
  if (mobileSort)
    mobileSort.addEventListener("change", (e) => {
      state.sortBy = e.target.value;
      state.sortOrder = "asc";
      state.lastETag = null;
      loadReminders(true);
    });

  if (pageSizeSelect)
    pageSizeSelect.addEventListener("change", (e) => {
      state.pageSize = parseInt(e.target.value);
      state.lastETag = null;
      loadReminders(true);
    });

  let searchTimeout;
  if (searchInput)
    searchInput.addEventListener("input", (e) => {
      clearTimeout(searchTimeout);
      state.searchTerm = e.target.value.trim();
      searchTimeout = setTimeout(() => {
        state.lastETag = null;
        loadReminders(true);
      }, 300);
    });

  const refreshBtn = document.getElementById("refresh-btn");
  if (refreshBtn)
    refreshBtn.addEventListener("click", () => {
      state.lastETag = null;
      loadReminders(false);
      import("./toast.js").then((m) => m.showMsg(t("dataRefreshed")));
    });

  window.toggleReminder = async (id) => {
    try {
      const res = await fetch(`/api/reminders/${id}/toggle`, {
        method: "PATCH",
      });
      if (!res.ok) {
        import("./toast.js").then((m) => m.showMsg(t("toggleFailed"), true));
        state.lastETag = null;
        loadReminders(false);
      } else {
        state.lastETag = null;
      }
    } catch (err) {
      import("./toast.js").then((m) =>
        m.showMsg("Gagal mengubah status", true),
      );
      state.lastETag = null;
      loadReminders(false);
    }
  };
}
