import {
  DeleteOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import { Button, Input, Popover, Segmented, Select, Tooltip } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import { detectSubscriptionIcon } from "../../../services/appBridge";
import type {
  SubscriptionIconKind,
  SubscriptionSourceKind,
} from "../../../types/subscription";
import { STRATEGY_GROUP_EMOJIS } from "../../local-config/strategyGroupEmoji";
import { SubscriptionIcon } from "./SubscriptionIcon";

interface SubscriptionIconEditorProps {
  icon: string;
  iconKind: SubscriptionIconKind;
  onChange: (iconKind: SubscriptionIconKind, icon: string) => void;
  onDetectionResult: (result: "found" | "not_found" | "error") => void;
  sourceKind?: SubscriptionSourceKind;
  sourceUrl: string;
}

export function SubscriptionIconEditor({
  icon,
  iconKind,
  onChange,
  onDetectionResult,
  sourceKind = "url",
  sourceUrl,
}: SubscriptionIconEditorProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [detecting, setDetecting] = useState(false);
  const [mode, setMode] = useState<"emoji" | "url">(
    iconKind === "url" ? "url" : "emoji",
  );
  const detectionRequest = useRef(0);
  const latestDraft = useRef({ icon, iconKind, sourceUrl });
  latestDraft.current = { icon, iconKind, sourceUrl };

  useEffect(() => {
    if (iconKind === "emoji" || iconKind === "url") setMode(iconKind);
  }, [iconKind]);

  useEffect(() => {
    detectionRequest.current += 1;
    setDetecting(false);
  }, [icon, iconKind, sourceUrl]);

  const detect = async () => {
    const requestedSource = sourceUrl.trim();
    if (!requestedSource) return;
    const requestedIcon = icon;
    const requestedIconKind = iconKind;
    const request = detectionRequest.current + 1;
    detectionRequest.current = request;
    setDetecting(true);
    try {
      const result = await detectSubscriptionIcon(requestedSource);
      if (
        request !== detectionRequest.current ||
        latestDraft.current.sourceUrl.trim() !== requestedSource ||
        latestDraft.current.icon !== requestedIcon ||
        latestDraft.current.iconKind !== requestedIconKind
      ) {
        return;
      }
      if (result.found) {
        setMode("url");
        onChange(result.iconKind, result.icon);
        onDetectionResult("found");
      } else {
        onDetectionResult("not_found");
      }
    } catch {
      if (request === detectionRequest.current) onDetectionResult("error");
    } finally {
      if (request === detectionRequest.current) setDetecting(false);
    }
  };

  const editor = (
    <div className="subscription-icon-popover">
      <Segmented
        aria-label={t("subscription.form.iconMode")}
        block
        onChange={(value) => {
          const next = value as "emoji" | "url";
          setMode(next);
          if (icon) onChange(next, "");
        }}
        options={[
          { label: t("subscription.form.iconEmoji"), value: "emoji" },
          { label: t("subscription.form.iconUrl"), value: "url" },
        ]}
        value={mode}
      />
      {mode === "emoji" ? (
        <Select
          allowClear
          aria-label={t("subscription.form.iconEmoji")}
          onChange={(value) => onChange(value ? "emoji" : "", value ?? "")}
          options={STRATEGY_GROUP_EMOJIS.map((emoji) => ({
            label: emoji,
            value: emoji,
          }))}
          placeholder={t("subscription.form.iconEmojiPlaceholder")}
          showSearch
          value={iconKind === "emoji" && icon ? icon : undefined}
        />
      ) : (
        <Input
          aria-label={t("subscription.form.iconUrl")}
          onChange={(event) =>
            onChange(event.target.value ? "url" : "", event.target.value)
          }
          placeholder="https://example.com/favicon.ico"
          value={iconKind === "url" ? icon : ""}
        />
      )}
      <div className="subscription-icon-popover-actions">
        {!icon ? (
          <Button
            disabled={!sourceUrl.trim()}
            icon={<SearchOutlined />}
            loading={detecting}
            onClick={() => void detect()}
            size="small"
          >
            {t("subscription.form.iconDetect")}
          </Button>
        ) : null}
        <Button
          disabled={!icon}
          icon={<DeleteOutlined />}
          onClick={() => onChange("", "")}
          size="small"
          type="text"
        >
          {t("subscription.form.iconClear")}
        </Button>
      </div>
    </div>
  );

  return (
    <div className="subscription-icon-editor-field">
      <span className="subscription-icon-editor-label">
        <span>{t("subscription.form.icon")}</span>
        <FeatureHelp compact topic="subscriptionIcon" />
      </span>
      <Popover
        content={editor}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (nextOpen && !icon && sourceUrl.trim() && !detecting) {
            void detect();
          }
        }}
        open={open}
        placement="bottomLeft"
        trigger="click"
      >
        <Tooltip title={t("subscription.form.iconEdit")}>
          <Button
            aria-label={t("subscription.form.iconEdit")}
            className="subscription-icon-editor-trigger"
          >
            <SubscriptionIcon
              icon={icon}
              iconKind={iconKind}
              sourceKind={sourceKind}
            />
          </Button>
        </Tooltip>
      </Popover>
    </div>
  );
}
