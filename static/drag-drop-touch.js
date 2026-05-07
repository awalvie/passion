/**
 * Minimal Touch-to-Drag-and-Drop polyfill.
 * Translates touchstart/touchmove/touchend into dragstart/drag/dragenter/dragleave/dragover/drop/dragend
 * so that HTML5 DnD code works on mobile browsers without modification.
 *
 * Only activates on touch devices. Requires elements with [draggable="true"].
 */
(function () {
  "use strict";

  // Only polyfill on touch devices
  if (!("ontouchstart" in document)) return;

  var DragDropTouch = (function () {
    function DragDropTouch() {
      this._dragSource = null;
      this._lastTouch = null;
      this._lastTarget = null;
      this._dataTransfer = null;
      this._img = null;
      this._imgOffset = { x: 0, y: 0 };
      this._dragStartThreshold = 5; // px before drag starts

      // Listen to touch events on the document
      document.addEventListener("touchstart", this._touchstart.bind(this), { passive: false });
      document.addEventListener("touchmove", this._touchmove.bind(this), { passive: false });
      document.addEventListener("touchend", this._touchend.bind(this));
    }

    DragDropTouch.prototype._touchstart = function (e) {
      if (e.touches.length !== 1) return;
      var touch = e.touches[0];
      var src = this._closestDraggable(touch.target);
      if (!src) return;
      this._dragSource = src;
      this._lastTouch = touch;
      this._startPoint = { x: touch.clientX, y: touch.clientY };
      this._isDragging = false;
    };

    DragDropTouch.prototype._touchmove = function (e) {
      if (!this._dragSource) return;
      var touch = e.touches[0];
      this._lastTouch = touch;

      if (!this._isDragging) {
        var dx = touch.clientX - this._startPoint.x;
        var dy = touch.clientY - this._startPoint.y;
        if (Math.abs(dx) + Math.abs(dy) < this._dragStartThreshold) return;
        this._isDragging = true;
        this._dataTransfer = new DataTransferShim();
        if (!this._dispatchEvent(this._dragSource, "dragstart", touch)) {
          this._reset();
          return;
        }
        this._createGhost(this._dragSource, touch);
      }

      e.preventDefault(); // Prevent scrolling during drag

      this._moveGhost(touch);
      this._dispatchEvent(this._dragSource, "drag", touch);

      var target = this._getDropTarget(touch);
      if (target !== this._lastTarget) {
        if (this._lastTarget) this._dispatchEvent(this._lastTarget, "dragleave", touch);
        if (target) this._dispatchEvent(target, "dragenter", touch);
        this._lastTarget = target;
      }
      if (target) this._dispatchEvent(target, "dragover", touch);
    };

    DragDropTouch.prototype._touchend = function (e) {
      if (!this._dragSource) return;

      if (this._isDragging) {
        var touch = this._lastTouch;
        var target = this._getDropTarget(touch);
        if (target) {
          this._dispatchEvent(target, "drop", touch);
        }
        this._dispatchEvent(this._dragSource, "dragend", touch);
        this._destroyGhost();
      }
      this._reset();
    };

    DragDropTouch.prototype._reset = function () {
      this._dragSource = null;
      this._lastTouch = null;
      this._lastTarget = null;
      this._dataTransfer = null;
      this._isDragging = false;
    };

    DragDropTouch.prototype._closestDraggable = function (el) {
      while (el) {
        if (el.getAttribute && el.getAttribute("draggable") === "true") return el;
        // Also check for dnd-handle: if user touches a handle, find the draggable parent
        if (el.classList && el.classList.contains("dnd-handle")) {
          var parent = el.closest("[draggable='true']");
          if (parent) return parent;
        }
        el = el.parentElement;
      }
      return null;
    };

    DragDropTouch.prototype._getDropTarget = function (touch) {
      // Temporarily hide the ghost so elementFromPoint finds the real target
      if (this._img) this._img.style.display = "none";
      var target = document.elementFromPoint(touch.clientX, touch.clientY);
      if (this._img) this._img.style.display = "";
      // Walk up to find a droppable element
      while (target) {
        if (target === this._dragSource) return target;
        // Accept any element that listens for dragover/drop (we can't know, so accept all)
        if (target.nodeType === 1) return target;
        target = target.parentElement;
      }
      return null;
    };

    DragDropTouch.prototype._dispatchEvent = function (target, type, touch) {
      if (!target) return true;
      var evt = new Event(type, { bubbles: true, cancelable: true });
      evt.dataTransfer = this._dataTransfer;
      evt.clientX = touch.clientX;
      evt.clientY = touch.clientY;
      evt.pageX = touch.pageX;
      evt.pageY = touch.pageY;
      evt.screenX = touch.screenX;
      evt.screenY = touch.screenY;
      return target.dispatchEvent(evt);
    };

    DragDropTouch.prototype._createGhost = function (src, touch) {
      this._img = src.cloneNode(true);
      var s = this._img.style;
      var rect = src.getBoundingClientRect();
      s.position = "fixed";
      s.pointerEvents = "none";
      s.zIndex = "99999";
      s.width = rect.width + "px";
      s.opacity = "0.7";
      s.transform = "rotate(2deg)";
      s.transition = "none";
      this._imgOffset = {
        x: touch.clientX - rect.left,
        y: touch.clientY - rect.top,
      };
      this._moveGhost(touch);
      document.body.appendChild(this._img);
    };

    DragDropTouch.prototype._moveGhost = function (touch) {
      if (!this._img) return;
      this._img.style.left = (touch.clientX - this._imgOffset.x) + "px";
      this._img.style.top = (touch.clientY - this._imgOffset.y) + "px";
    };

    DragDropTouch.prototype._destroyGhost = function () {
      if (this._img && this._img.parentElement) {
        this._img.parentElement.removeChild(this._img);
      }
      this._img = null;
    };

    return DragDropTouch;
  })();

  // Simple DataTransfer shim
  function DataTransferShim() {
    this._data = {};
    this.dropEffect = "move";
    this.effectAllowed = "all";
    this.types = [];
  }
  DataTransferShim.prototype.setData = function (type, val) {
    this._data[type] = val;
    if (this.types.indexOf(type) < 0) this.types.push(type);
  };
  DataTransferShim.prototype.getData = function (type) {
    return this._data[type] || "";
  };
  DataTransferShim.prototype.clearData = function (type) {
    if (type) {
      delete this._data[type];
      var idx = this.types.indexOf(type);
      if (idx >= 0) this.types.splice(idx, 1);
    } else {
      this._data = {};
      this.types = [];
    }
  };
  DataTransferShim.prototype.setDragImage = function () {
    // no-op: we use our own ghost
  };

  // Initialize
  new DragDropTouch();
})();
