const tooltip = document.createElement("div");
tooltip.className = "tooltip";
tooltip.hidden = true;
document.body.append(tooltip);

function fill(el) {
  const data = JSON.parse(el.dataset.tip);
  const title = document.createElement("div");
  title.className = "tt-title";
  title.textContent = data.title;
  tooltip.replaceChildren(title);
  for (const [value, label, key] of data.rows) {
    const row = document.createElement("div");
    row.className = "tt-row";
    if (key) {
      const line = document.createElement("span");
      line.className = "key " + key;
      row.append(line);
    }
    const strong = document.createElement("strong");
    strong.textContent = value;
    const name = document.createElement("span");
    name.className = "muted";
    name.textContent = label;
    row.append(strong, name);
    tooltip.append(row);
  }
  tooltip.hidden = false;
}

function place(x, y) {
  const pad = 12;
  const { width, height } = tooltip.getBoundingClientRect();
  let left = x + pad;
  let top = y - height - pad;
  if (left + width > window.innerWidth - pad) left = x - width - pad;
  if (top < pad) top = y + pad;
  tooltip.style.left = Math.max(pad, left) + "px";
  tooltip.style.top = top + "px";
}

document.addEventListener("pointermove", (event) => {
  const el = event.target.closest("[data-tip]");
  if (!el) {
    tooltip.hidden = true;
    return;
  }
  fill(el);
  place(event.clientX, event.clientY);
});

document.addEventListener("pointerleave", () => { tooltip.hidden = true; });

document.addEventListener("focusin", (event) => {
  const el = event.target.closest("[data-tip]");
  if (!el) return;
  fill(el);
  const rect = el.getBoundingClientRect();
  place(rect.left + rect.width / 2, rect.top);
});

document.addEventListener("focusout", () => { tooltip.hidden = true; });
