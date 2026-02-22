import { t, fpLocaleId, currentLang } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { state, globals } from "../store/state.js";
import { loadReminders } from "./reminders-table.js";
import { htmlToWAMarkdown, formatWhatsAppMarkdown } from "../utils/html.js";
import { cronToText, cronNextTime } from "../utils/format.js";
import { renderEditTargetChips } from "./target-chips.js";

const editModal = document.getElementById("edit-modal");
const closeEditBtn = document.getElementById("close-edit-btn");
const editForm = document.getElementById("edit-schedule-form");
const editTimeContainer = document.getElementById("edit-time-container");
const editRecurrenceInput = document.getElementById("edit-recurrence");
const editCronPreview = document.getElementById("edit-cron-preview");

export let editTimePicker;

export function initEditModal() {
  editTimePicker = flatpickr("#edit-time", {
    enableTime: true,
    time_24hr: true,
    dateFormat: "Y-m-d H:i",
    altInput: true,
    altFormat: "j F Y, H:i",
    minDate: "today",
    disableMobile: true,
    appendTo: document.body,
    locale: currentLang === "id" ? fpLocaleId : "default",
  });

  function updateEditTimeVisibility() {
    const hasCron = editRecurrenceInput.value.trim() !== "";
    editTimeContainer.hidden = hasCron;
  }

  if (editRecurrenceInput)
    editRecurrenceInput.addEventListener("input", (e) => {
      const expr = e.target.value.trim();
      const text = cronToText(expr, true);
      const next = cronNextTime(expr);
      editCronPreview.textContent = next ? `${text} — ${next}` : text;
      updateEditTimeVisibility();
    });

  document.querySelectorAll(".edit-cron-preset").forEach((btn) => {
    btn.addEventListener("click", () => {
      editRecurrenceInput.value = btn.dataset.cron;
      const expr = btn.dataset.cron;
      const text = cronToText(expr, true);
      const next = cronNextTime(expr);
      editCronPreview.textContent = next ? `${text} — ${next}` : text;
      updateEditTimeVisibility();
    });
  });

  window.editReminder = (id) => {
    const rem = state.remindersData.find((r) => r.id === id);
    if (!rem) {
      showMsg(t("editNotFound"), true);
      return;
    }

    document.getElementById("edit-id").value = rem.id;
    if (globals.editQuill) {
      globals.editQuill.root.innerHTML = formatWhatsAppMarkdown(rem.message);
    }

    globals.editTargetNumbers = rem.target_wa
      ? rem.target_wa
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean)
      : [];
    renderEditTargetChips();
    document.getElementById("edit-target-input").value = "";

    editRecurrenceInput.value = rem.recurrence || "";

    if (rem.recurrence) {
      const text = cronToText(rem.recurrence);
      const next = cronNextTime(rem.recurrence);
      editCronPreview.textContent = next ? `${text} — ${next}` : text;
      editTimePicker.clear();
    } else {
      editCronPreview.textContent = t("cronHintEmpty");
      editTimePicker.setDate(new Date(rem.scheduled_at));
    }

    updateEditTimeVisibility();
    editModal.classList.add("active");
    document.body.style.overflow = "hidden";
  };

  if (closeEditBtn)
    closeEditBtn.addEventListener("click", () => {
      editModal.classList.remove("active");
      document.body.style.overflow = "";
    });

  if (editModal)
    editModal.addEventListener("click", (e) => {
      if (e.target === editModal) closeEditBtn.click();
    });

  if (editForm)
    editForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const id = document.getElementById("edit-id").value;
      const btn = document.getElementById("edit-save-btn");

      const message = globals.editQuill
        ? htmlToWAMarkdown(globals.editQuill.root.innerHTML)
        : "";
      const targetWa = (globals.editTargetNumbers || []).join(",");
      const recurrence = editRecurrenceInput.value;
      const timeVal = document.getElementById("edit-time").value;
      const hasCron = recurrence.trim() !== "";

      if (!hasCron && !timeVal) {
        showMsg(t("editTimePast"), true);
        return;
      }

      try {
        btn.disabled = true;
        const isoDate = hasCron
          ? new Date().toISOString()
          : new Date(timeVal).toISOString();

        const payload = {
          id: id,
          message: message,
          target_wa: targetWa,
          recurrence: recurrence.trim(),
          scheduled_at: isoDate,
        };

        const res = await fetch(`/api/reminders/${id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });

        if (res.ok) {
          editModal.classList.remove("active");
          document.body.style.overflow = "";
          showMsg(t("editSuccess"));
          state.lastETag = null;
          loadReminders(false);
        } else {
          const err = await res.text();
          showMsg(`${t("editFailed")} ${err}`, true);
        }
      } catch (err) {
        showMsg(t("editNetError"), true);
      } finally {
        btn.disabled = false;
      }
    });
}
