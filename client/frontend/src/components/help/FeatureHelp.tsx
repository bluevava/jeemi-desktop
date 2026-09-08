import {
  BulbOutlined,
  CompassOutlined,
  InfoCircleOutlined,
  QuestionCircleOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { Button, Popover } from "antd";
import { useEffect, useRef, useState, type ComponentRef } from "react";
import { useTranslation } from "react-i18next";
import type { zhCN } from "../../i18n/locales/zh-CN";

export type HelpTopic = keyof typeof zhCN.help.topics;

interface FeatureHelpProps {
  topic?: HelpTopic;
  translationBase?: string;
  fallbackBase?: string;
  values?: Record<string, string | number>;
  compact?: boolean;
}

const sections = [
  { key: "description", icon: <InfoCircleOutlined /> },
  { key: "purpose", icon: <BulbOutlined /> },
  { key: "scenarios", icon: <CompassOutlined /> },
  { key: "cautions", icon: <WarningOutlined /> },
] as const;

export function FeatureHelp({
  topic,
  translationBase,
  fallbackBase,
  values,
  compact = false,
}: FeatureHelpProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<ComponentRef<typeof Button>>(null);
  useEffect(() => {
    if (!open) return;
    const dismiss = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
      triggerRef.current?.focus();
    };
    document.addEventListener("keydown", dismiss, true);
    return () => document.removeEventListener("keydown", dismiss, true);
  }, [open]);
  const base =
    translationBase ??
    (topic ? `help.topics.${topic}` : fallbackBase ?? "help.topics.localConfigField");
  const translate = (key: string) =>
    t(`${base}.${key}`, {
      ...values,
      defaultValue: fallbackBase
        ? t(`${fallbackBase}.${key}`, values)
        : t(`help.topics.localConfigField.${key}`, values),
    });

  const content = (
    <div className="feature-help-content">
      {sections.map((section) => (
        <section className="feature-help-section" key={section.key}>
          <div className="feature-help-section-title">
            {section.icon}
            <span>{t(`help.section.${section.key}`)}</span>
          </div>
          <p>{translate(section.key)}</p>
        </section>
      ))}
    </div>
  );

  return (
    <Popover
      arrow
      content={content}
      onOpenChange={setOpen}
      open={open}
      placement="bottom"
      title={translate("title")}
      trigger="click"
    >
      <Button
        aria-expanded={open}
        aria-label={`${t("common.learnMore")}: ${translate("title")}`}
        className={compact ? "feature-help-button compact" : "feature-help-button"}
        icon={<QuestionCircleOutlined />}
        ref={triggerRef}
        size={compact ? "small" : "middle"}
        type="text"
      />
    </Popover>
  );
}
