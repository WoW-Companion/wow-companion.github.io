/* Click-to-enlarge for screenshots. No libraries. Without JS the image links to its file. */
(function () {
  var links = document.querySelectorAll("a.zoom");
  if (!links.length) return;
  var box, img, fullBtn, closeBtn, opener;
  function build() {
    box = document.createElement("div");
    box.className = "lb";
    box.hidden = true;
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-modal", "true");
    box.setAttribute("aria-label", "Screenshot");
    box.innerHTML = '<div class="lb-bar"><button type="button" class="lb-full" hidden>Full size</button><button type="button" class="lb-close" aria-label="Close">\u00d7 Close</button></div><div class="lb-stage"><img alt=""></div>';
    document.body.appendChild(box);
    img = box.querySelector("img");
    fullBtn = box.querySelector(".lb-full");
    closeBtn = box.querySelector(".lb-close");
    closeBtn.addEventListener("click", close);
    fullBtn.addEventListener("click", toggle);
    img.addEventListener("click", function () { if (!fullBtn.hidden) toggle(); });
    box.addEventListener("click", function (e) { if (e.target === box || e.target.className === "lb-stage" || e.target.className === "lb-bar") close(); });
    box.addEventListener("keydown", function (e) {
      if (e.key === "Escape") { e.preventDefault(); close(); return; }
      if (e.key !== "Tab") return;
      var f = [closeBtn]; if (!fullBtn.hidden) f.unshift(fullBtn);
      var i = f.indexOf(document.activeElement);
      e.preventDefault();
      f[(i + (e.shiftKey ? -1 : 1) + f.length) % f.length].focus();
    });
  }
  function canFull() {
    var s = img.parentNode;
    return img.naturalWidth > s.clientWidth - 16 || img.naturalHeight > s.clientHeight - 16;
  }
  function update() {
    var full = box.classList.contains("full");
    fullBtn.hidden = !full && !canFull();
    fullBtn.textContent = full ? "Fit to window" : "Full size";
  }
  function toggle() { box.classList.toggle("full"); update(); }
  function open(a) {
    if (!box) build();
    opener = a;
    var t = a.querySelector("img");
    box.classList.remove("full");
    img.alt = t ? t.alt : "";
    box.setAttribute("aria-label", img.alt || "Screenshot");
    img.onload = update;
    img.src = a.getAttribute("href");
    box.hidden = false;
    document.documentElement.style.overflow = "hidden";
    update();
    closeBtn.focus();
  }
  function close() {
    box.hidden = true;
    img.removeAttribute("src");
    document.documentElement.style.overflow = "";
    if (opener) opener.focus();
  }
  Array.prototype.forEach.call(links, function (a) {
    a.setAttribute("role", "button");
    a.setAttribute("aria-haspopup", "dialog");
    a.addEventListener("click", function (e) {
      if (e.ctrlKey || e.metaKey || e.shiftKey || e.button) return;
      e.preventDefault(); open(a);
    });
    a.addEventListener("keydown", function (e) {
      if (e.key === " ") { e.preventDefault(); open(a); }
    });
  });
})();
