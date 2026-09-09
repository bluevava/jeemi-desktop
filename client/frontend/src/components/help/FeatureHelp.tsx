import {
  BulbOutlined,
  CodeOutlined,
  CompassOutlined,
  QuestionCircleOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { Button, Popover } from "antd";
import { useEffect, useRef, useState, type ComponentRef } from "react";
import { useTranslation } from "react-i18next";
import type { zhCN } from "../../i18n/locales/zh-CN";
import { helpSections, resolveHelpContent } from "./helpContent";

export type HelpTopic = keyof typeof zhCN.help.topics;

interface FeatureHelpProps {
  topic?: HelpTopic;
  translationBase?: string;
  fallbackBase?: string;
  values?: Record<string, string | number>;
  compact?: boolean;
}

const sectionIcons = {
  purpose: <BulbOutlined />,
  scenarios: <CompassOutlined />,
  cautions: <WarningOutlined />,
  example: <CodeOutlined />,
  cautionsAndExample: <WarningOutlined />,
};

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
  const topicContent = resolveHelpContent(t, base, fallbackBase, values);

  const content = (
    <div className="feature-help-content">
      {helpSections(topicContent).map((section) => (
        <section
          className={`feature-help-section${section.caution ? " feature-help-caution" : ""}`}
          key={section.key}
        >
          <div className="feature-help-section-title">
            {sectionIcons[section.key]}
            <span>{t(`help.section.${section.key}`)}</span>
          </div>
          {section.paragraphs.map((paragraph, index) => (
            <p key={index}>{paragraph}</p>
          ))}
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
      title={topicContent.title}
      trigger="click"
    >
      <Button
        aria-expanded={open}
        aria-label={`${t("common.learnMore")}: ${topicContent.title}`}
        className={compact ? "feature-help-button compact" : "feature-help-button"}
        icon={<QuestionCircleOutlined />}
        ref={triggerRef}
        size={compact ? "small" : "middle"}
        type="text"
      />
    </Popover>
  );
}
