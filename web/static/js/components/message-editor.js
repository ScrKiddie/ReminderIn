import { t } from "../i18n/lang.js";
import { globals } from "../store/state.js";

const messageList = document.getElementById("message-list");
const addMessageBtn = document.getElementById("add-message-btn");

export function createLiteEditor(containerId) {
  const container = document.getElementById(containerId);
  if (!container) return null;
  container.innerHTML = "";

  const wrapper = document.createElement("div");
  wrapper.className = "lite-editor";

  const toolbar = document.createElement("div");
  toolbar.className = "lite-toolbar";
  const cmds = [
    { cmd: "bold", label: "<b>B</b>" },
    { cmd: "italic", label: "<i>I</i>" },
    { cmd: "strikeThrough", label: "<s>S</s>" },
    { cmd: "removeFormat", label: "T\u0338" },
  ];
  cmds.forEach((c) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.innerHTML = c.label;
    btn.title = c.cmd;
    btn.addEventListener("mousedown", (e) => {
      e.preventDefault();
      document.execCommand(c.cmd, false, null);
    });
    toolbar.appendChild(btn);
  });

  const content = document.createElement("div");
  content.className = "lite-content";
  content.contentEditable = "true";
  content.setAttribute("data-placeholder", t("typeMessagePlaceholder"));

  content.addEventListener("input", () => {
    if (
      content.innerHTML === "<br>" ||
      content.innerHTML === "<div><br></div>" ||
      content.innerText.trim() === ""
    ) {
      content.innerHTML = "";
    }
  });

  wrapper.appendChild(toolbar);
  wrapper.appendChild(content);
  container.appendChild(wrapper);

  return { root: content };
}

export function initQuill(id) {
  globals.messageEditors[id] = createLiteEditor(`message-container-${id}`);
}

export function pruneMessageEditors() {
  Object.keys(globals.messageEditors).forEach((key) => {
    if (!document.getElementById(`message-container-${key}`)) {
      delete globals.messageEditors[key];
    }
  });
}

export function initMessageEditor() {
  if (addMessageBtn)
    addMessageBtn.addEventListener("click", () => {
      const newId = `message-${globals.messageCount}`;
      const block = document.createElement("div");
      block.className = "message-block";
      block.style.marginBottom = "5px";

      const currentBlocks =
        messageList.querySelectorAll(".message-block").length;

      block.innerHTML = `
        <label for="message-${globals.messageCount}"><span data-i18n="messageLabel">${t("messageLabel")}</span> ${currentBlocks + 1}:</label>
        <div id="message-container-${globals.messageCount}"></div>
        <button type="button" class="remove-message-btn" style="color: red; margin-top: 5px;" data-i18n="removeMessage">${t("removeMessage")}</button>
    `;
      messageList.appendChild(block);

      initQuill(globals.messageCount);

      const removeBtns = messageList.querySelectorAll(".remove-message-btn");
      removeBtns.forEach((b) => (b.style.display = "inline-block"));

      globals.messageCount++;
    });

  if (messageList)
    messageList.addEventListener("click", (e) => {
      if (e.target.classList.contains("remove-message-btn")) {
        const block = e.target.closest(".message-block");
        if (block) {
          const container = block.querySelector(
            "div[id^='message-container-']",
          );
          if (container) {
            const key = container.id.replace("message-container-", "");
            delete globals.messageEditors[key];
          }
          block.remove();
        }
        const removeBtns = messageList.querySelectorAll(".remove-message-btn");
        if (removeBtns.length === 1) {
          removeBtns[0].style.display = "none";
        }

        pruneMessageEditors();

        const blocks = messageList.querySelectorAll(".message-block");
        blocks.forEach((b, index) => {
          b.querySelector("label").innerHTML =
            `<span data-i18n="messageLabel">${t("messageLabel")}</span> ${index + 1}:`;
        });
      }
    });
}
