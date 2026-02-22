import { t } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { globals } from "../store/state.js";
import { renderTargetChips, renderEditTargetChips } from "./target-chips.js";

const pickerModal = document.getElementById("picker-modal");
const modalTitle = document.getElementById("modal-title");
const modalSearch = document.getElementById("modal-search");
const modalList = document.getElementById("modal-list");
const modalCloseBtn = document.getElementById("modal-close-btn");
const pickGroupBtn = document.getElementById("pick-group-btn");
const pickContactBtn = document.getElementById("pick-contact-btn");
const editPickGroupBtn = document.getElementById("edit-pick-group-btn");
const editPickContactBtn = document.getElementById("edit-pick-contact-btn");

let modalItems = [];
let currentPickerTarget = "schedule";

export function openModal(title, items, target = "schedule") {
  modalTitle.textContent = title;
  modalItems = items;
  currentPickerTarget = target;
  modalSearch.value = "";
  renderModalList("");
  pickerModal.classList.add("active");
  document.body.style.overflow = "hidden";
  modalSearch.focus();
}

export function closeModal() {
  pickerModal.classList.remove("active");
  document.body.style.overflow = "";
}

function renderModalList(filter) {
  modalList.innerHTML = "";
  const lower = filter.toLowerCase();
  const filtered = modalItems.filter((it) =>
    it.label.toLowerCase().includes(lower),
  );

  if (filtered.length === 0) {
    const empty = document.createElement("div");
    empty.style.padding = "15px";
    empty.style.textAlign = "center";
    empty.style.opacity = "0.5";
    empty.textContent = t("noContactsFound");
    modalList.appendChild(empty);
    return;
  }

  filtered.forEach((it) => {
    const div = document.createElement("div");
    div.className = "modal-list-item";

    const meta = document.createElement("div");
    meta.style.flex = "1";

    const label = document.createElement("strong");
    label.style.display = "block";
    label.textContent = it.label;

    const jid = document.createElement("small");
    jid.style.opacity = "0.7";
    jid.textContent = it.jid;

    const pickBtn = document.createElement("button");
    pickBtn.className = "pick-btn";
    pickBtn.style.padding = "5px 10px";
    pickBtn.textContent = t("pick");

    meta.appendChild(label);
    meta.appendChild(jid);
    div.appendChild(meta);
    div.appendChild(pickBtn);

    pickBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      if (currentPickerTarget === "edit") {
        if (globals.editTargetNumbers.includes(it.jid)) {
          showMsg(t("alreadyAdded"), true);
          return;
        }
        globals.editTargetNumbers.push(it.jid);
        renderEditTargetChips();
      } else {
        if (globals.targetNumbers.includes(it.jid)) {
          showMsg(t("alreadyAdded"), true);
          return;
        }
        globals.targetNumbers.push(it.jid);
        renderTargetChips();
      }
      closeModal();
    });
    div.addEventListener("click", () => {
      pickBtn.click();
    });
    modalList.appendChild(div);
  });
}

export function initPickerModal() {
  if (modalSearch)
    modalSearch.addEventListener("input", (e) => {
      renderModalList(e.target.value);
    });

  if (modalCloseBtn) modalCloseBtn.addEventListener("click", closeModal);
  if (pickerModal)
    pickerModal.addEventListener("click", (e) => {
      if (e.target === pickerModal) closeModal();
    });

  if (pickGroupBtn)
    pickGroupBtn.addEventListener("click", async () => {
      pickGroupBtn.disabled = true;
      pickGroupBtn.textContent = t("loading");
      try {
        const res = await fetch("/api/wa/groups");
        const groups = await res.json();
        globals.groupsCache = {};

        if (groups.length === 0) {
          showMsg(t("noGroupsFound"), true);
          return;
        }

        const items = groups.map((g) => {
          globals.groupsCache[g.jid] = g.name;
          return { jid: g.jid, label: g.name };
        });

        openModal(t("pickGroup"), items);
      } catch (err) {
        showMsg(t("loadGroupsFailed"), true);
      } finally {
        pickGroupBtn.disabled = false;
        pickGroupBtn.textContent = t("pickGroup");
      }
    });

  if (pickContactBtn)
    pickContactBtn.addEventListener("click", async () => {
      pickContactBtn.disabled = true;
      pickContactBtn.textContent = t("loading");
      try {
        const res = await fetch("/api/wa/contacts");
        const contacts = await res.json();
        globals.contactsCache = {};

        if (contacts.length === 0) {
          showMsg(t("noContactsFound"), true);
          return;
        }

        const items = contacts.map((c) => {
          globals.contactsCache[c.jid] = c.name;
          return { jid: c.jid, label: `${c.name} (${c.jid})` };
        });

        openModal(t("pickContact"), items);
      } catch (err) {
        showMsg(t("loadContactsFailed"), true);
      } finally {
        pickContactBtn.disabled = false;
        pickContactBtn.textContent = t("pickContact");
      }
    });

  if (editPickGroupBtn)
    editPickGroupBtn.addEventListener("click", async () => {
      editPickGroupBtn.disabled = true;
      editPickGroupBtn.textContent = t("loading");
      try {
        const res = await fetch("/api/wa/groups");
        const groups = await res.json();
        globals.groupsCache = globals.groupsCache || {};

        if (groups.length === 0) {
          showMsg(t("noGroupsFound"), true);
          return;
        }

        const items = groups.map((g) => {
          globals.groupsCache[g.jid] = g.name;
          return { jid: g.jid, label: g.name };
        });

        openModal(t("pickGroup"), items, "edit");
      } catch (err) {
        showMsg(t("loadGroupsFailed"), true);
      } finally {
        editPickGroupBtn.disabled = false;
        editPickGroupBtn.textContent = t("pickGroup");
      }
    });

  if (editPickContactBtn)
    editPickContactBtn.addEventListener("click", async () => {
      editPickContactBtn.disabled = true;
      editPickContactBtn.textContent = t("loading");
      try {
        const res = await fetch("/api/wa/contacts");
        const contacts = await res.json();
        globals.contactsCache = globals.contactsCache || {};

        if (contacts.length === 0) {
          showMsg(t("noContactsFound"), true);
          return;
        }

        const items = contacts.map((c) => {
          globals.contactsCache[c.jid] = c.name;
          return { jid: c.jid, label: `${c.name} (${c.jid})` };
        });

        openModal(t("pickContact"), items, "edit");
      } catch (err) {
        showMsg(t("loadContactsFailed"), true);
      } finally {
        editPickContactBtn.disabled = false;
        editPickContactBtn.textContent = t("pickContact");
      }
    });
}
