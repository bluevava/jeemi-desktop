import type { ReactNode } from "react";
import { PlusOutlined } from "@ant-design/icons";

interface LibraryCreateCardProps {
  className: string;
  help: ReactNode;
  onCreate: () => void;
  title: string;
}

export function LibraryCreateCard({
  className,
  help,
  onCreate,
  title,
}: LibraryCreateCardProps) {
  return (
    <div className={`library-create-card ${className}`}>
      {/* Separate buttons keep help independent from the full-card create action. */}
      <button
        aria-label={title}
        className="library-create-action"
        onClick={onCreate}
        type="button"
      />
      <span aria-hidden="true" className="library-create-icon">
        <PlusOutlined />
      </span>
      <span className="library-create-label">
        <strong>{title}</strong>
        {help}
      </span>
    </div>
  );
}
