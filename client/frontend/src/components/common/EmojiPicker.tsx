import { useState } from "react";
import { Button, Empty, Input, Popover, Select, Tooltip } from "antd";
import { AppstoreOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { EMOJI_CATEGORIES, filterEmojiOptions } from "../../lib/emojiCatalog";
import { FlagText } from "./FlagText";

export interface EmojiPickerProps {
  value: string;
  onChange: (value: string) => void;
}

// Shared grid only: subscription URL detection remains in its own editor.
export function EmojiPicker({ value, onChange }: EmojiPickerProps) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  const zh = i18n.language.startsWith("zh");
  const options = filterEmojiOptions(category, query);
  return (
    <div className="emoji-picker">
      <div className="emoji-picker-tools">
        <Input
          allowClear
          aria-label={t("resourceLibrary.icons.search")}
          placeholder={t("resourceLibrary.icons.search")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Select
          aria-label={t("resourceLibrary.icons.category")}
          value={category}
          onChange={setCategory}
          options={[
            { value: "", label: t("resourceLibrary.icons.all") },
            ...EMOJI_CATEGORIES.map((item) => ({
              value: item.id,
              label: zh ? item.zh : item.en,
            })),
          ]}
        />
      </div>
      <div
        className="emoji-picker-grid"
        aria-label={t("resourceLibrary.icons.options")}
      >
        {options.map((item) => (
          <Tooltip key={item.value} title={zh ? item.zh : item.en}>
            <Button
              aria-label={zh ? item.zh : item.en}
              aria-pressed={value === item.value}
              className="emoji-picker-option"
              type={value === item.value ? "primary" : "text"}
              onClick={() => onChange(item.value)}
            >
              <FlagText>{item.value}</FlagText>
            </Button>
          </Tooltip>
        ))}
        {!options.length ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t("resourceLibrary.empty")}
          />
        ) : null}
      </div>
      <Button
        size="small"
        type="text"
        disabled={!value}
        onClick={() => onChange("")}
      >
        {t("resourceLibrary.icons.clear")}
      </Button>
    </div>
  );
}

export function EmojiPickerField(props: EmojiPickerProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <div className="config-resource-field emoji-picker-field">
      <span>{t("resourceLibrary.icons.label")}</span>
      <Popover
        trigger="click"
        placement="bottomLeft"
        open={open}
        onOpenChange={setOpen}
        content={
          <EmojiPicker
            {...props}
            onChange={(value) => {
              props.onChange(value);
              setOpen(false);
            }}
          />
        }
      >
        <Button
          className="emoji-picker-trigger"
          aria-label={t("resourceLibrary.icons.choose")}
          aria-expanded={open}
        >
          {props.value ? (
            <FlagText>{props.value}</FlagText>
          ) : (
            <AppstoreOutlined />
          )}
        </Button>
      </Popover>
    </div>
  );
}
