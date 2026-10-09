import { useRef, useState } from "react";
import type { ButtonHTMLAttributes, HTMLAttributes } from "react";

import { moveSubscriptionCard } from "./cardOrder";

type SortHandleProps = Pick<ButtonHTMLAttributes<HTMLButtonElement>,
  "disabled" | "draggable" | "onClick" | "onDragStart" | "onDragEnd" | "onKeyDown">;

export function useSubscriptionCardDrag(ids: string[], disabled: boolean, onReorder: (ids: string[]) => void) {
  const dragged = useRef<string | null>(null);
  const [draggedID, setDraggedID] = useState<string | null>(null);
  const [targetID, setTargetID] = useState<string | null>(null);
  const unavailable = disabled || ids.length < 2;

  const reset = () => {
    dragged.current = null;
    setDraggedID(null);
    setTargetID(null);
  };
  const move = (source: string, target: string) => {
    if (unavailable) return;
    const next = moveSubscriptionCard(ids, source, target);
    if (next) onReorder(next);
  };

  const handleProps = (id: string): SortHandleProps => ({
    disabled: unavailable,
    draggable: !unavailable,
    onClick: (event) => event.stopPropagation(),
    onDragStart: (event) => {
      if (unavailable) {
        event.preventDefault();
        return;
      }
      event.stopPropagation();
      event.dataTransfer.effectAllowed = "move";
      event.dataTransfer.setData("text/plain", id);
      const card = event.currentTarget.closest<HTMLElement>(".subscription-card");
      if (card) event.dataTransfer.setDragImage(card, card.offsetWidth / 2, card.offsetHeight / 2);
      dragged.current = id;
      setDraggedID(id);
    },
    onDragEnd: reset,
    onKeyDown: (event) => {
      const direction = ["ArrowLeft", "ArrowUp"].includes(event.key) ? -1
        : ["ArrowRight", "ArrowDown"].includes(event.key) ? 1 : 0;
      if (!direction) return;
      event.preventDefault();
      event.stopPropagation();
      const target = ids[ids.indexOf(id) + direction];
      if (target) move(id, target);
    },
  });

  const dropProps = (id: string): HTMLAttributes<HTMLDivElement> => ({
    onDragOver: (event) => {
      if (unavailable || !dragged.current) return;
      event.preventDefault();
      event.stopPropagation();
      event.dataTransfer.dropEffect = "move";
      setTargetID(id === dragged.current ? null : id);
    },
    onDragLeave: (event) => {
      if (event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget)) return;
      setTargetID((current) => current === id ? null : current);
    },
    onDrop: (event) => {
      if (!dragged.current) return;
      event.preventDefault();
      event.stopPropagation();
      const source = dragged.current;
      reset();
      move(source, id);
    },
  });

  return { draggedID, targetID, handleProps, dropProps };
}
