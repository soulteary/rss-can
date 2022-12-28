(function () {
  function createHEP() {
    hep = new ElementPicker({ ignoreElements: [document.body, document.getElementById("rss-can-be-easier")] });
    document.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();

      if (hep._previousEvent && hep._previousEvent.path) {
        let selectors = [];
        for (let i = 0; i < hep._previousEvent.path.length; i++) {
          let el = hep._previousEvent.path[i];
          const tagname = (el.tagName || "").toLowerCase();
          const classname = (el.className || "").replace(" ", ".");
          const id = el.id;

          let selector = tagname;
          if (id) {
            selector = selector + "#" + id;
          } else if (classname) {
            selector = selector + "." + classname;
          }
          selectors.push(selector);
        }

        selectors = selectors.map((n) => n);

        if (selectors.length > 4) {
          selectors = selectors.slice(0, 4).reverse().join(" ");
        }
        console.log("choose elements: ");
        console.log(selectors);

        const all = document.querySelectorAll(selectors);
        console.log("similar elements count:", all.length);
        console.log(all);
      }
    });
  }

  function createPanel() {
    document.documentElement.style.userSelect = "none";

    let panel = document.createElement("div");
    panel.id = "rss-can-be-easier";
    panel.style.width = "300px";
    panel.style.height = "300px";
    panel.style.backgroundColor = "#000";
    panel.style.position = "absolute";
    panel.style.zIndex = "1000000000";
    panel.style.right = "50px";
    panel.style.top = "10%";

    panel.innerHTML = `
    <input type="text" placeholder="Type here" class="input input-bordered input-primary w-full max-w-xs" />    
    `;

    document.body.appendChild(panel);
    return panel;
  }

  function draggable(el) {
    el.addEventListener("mousedown", (e) => {
      const move = (px, py) => {
        el.style.left = px - x + "px";
        el.style.top = py - y + "px";
      };
      const onMouseMove = (e) => move(e.pageX, e.pageY);
      const onMouseUp = () => {
        document.removeEventListener("mousemove", onMouseMove);
        el.removeEventListener("mouseup", onMouseUp);
      };
      const x = e.clientX - el.getBoundingClientRect().left;
      const y = e.clientY - el.getBoundingClientRect().top;
      move(e.pageX, e.pageY, x, y);
      document.addEventListener("mousemove", onMouseMove);
      el.addEventListener("mouseup", onMouseUp);
      el.addEventListener("dragstart", () => false);
    });
  }

  let panel = createPanel();
  draggable(panel);
  createHEP();
})();
