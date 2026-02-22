import { t } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { state, globals } from "../store/state.js";
import { loadReminders } from "./reminders-table.js";
import { deleteAllRemindersApi } from "../api/reminders.js";

const deleteModal = document.getElementById("delete-modal");
const deleteModalTitle = document.getElementById("delete-modal-title");
const deleteModalMessage = document.getElementById("delete-modal-message");
const confirmDeleteBtn = document.getElementById("confirm-delete-btn");
const cancelDeleteBtn = document.getElementById("cancel-delete-btn");
const clearAllBtn = document.getElementById("clear-all-btn");

export function initDeleteModal() {
  window.deleteReminder = (id) => {
    globals.deleteId = id;
    globals.deleteMode = "single";
    deleteModalTitle.textContent = t("deleteTitle");
    deleteModalMessage.textContent = t("deleteMessage");
    deleteModal.classList.add("active");
    document.body.style.overflow = "hidden";
  };

  if (deleteModal)
    deleteModal.addEventListener("click", (e) => {
      if (e.target === deleteModal) cancelDeleteBtn.click();
    });

  if (clearAllBtn)
    clearAllBtn.addEventListener("click", () => {
      globals.deleteMode = "all";
      deleteModalTitle.textContent = t("deleteAll");
      deleteModalMessage.textContent = t("deleteAllMessage");
      deleteModal.classList.add("active");
      document.body.style.overflow = "hidden";
    });

  if (cancelDeleteBtn)
    cancelDeleteBtn.addEventListener("click", () => {
      deleteModal.classList.remove("active");
      document.body.style.overflow = "";
      globals.deleteId = null;
    });

  if (confirmDeleteBtn)
    confirmDeleteBtn.addEventListener("click", async () => {
      confirmDeleteBtn.disabled = true;
      confirmDeleteBtn.textContent = "Menghapus...";
      try {
        if (globals.deleteMode === "single") {
          if (!globals.deleteId) return;
          const res = await fetch(`/api/reminders/${globals.deleteId}`, {
            method: "DELETE",
          });
          if (res.ok) {
            const remindersList = document.getElementById("reminders-list");
            const row = remindersList.querySelector(
              `tr[data-id="${globals.deleteId}"]`,
            );
            if (row) row.remove();
            showMsg(t("deleted"));
            state.lastETag = null;
            loadReminders(false);
          }
        } else {
          const res = await deleteAllRemindersApi();
          if (res.ok) {
            showMsg(t("allDeleted"));
            state.lastETag = null;
            loadReminders(true);
          }
        }
        deleteModal.classList.remove("active");
        document.body.style.overflow = "";
      } catch (err) {
        showMsg(t("deleteFailed"), true);
      } finally {
        confirmDeleteBtn.disabled = false;
        confirmDeleteBtn.textContent = "Ya, Hapus";
        globals.deleteId = null;
      }
    });
}
