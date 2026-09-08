import { useEffect, useRef, useState } from "react";
import { Button, Input, Popover, Tooltip } from "antd";
import type { TextAreaRef } from "antd/es/input/TextArea";
import { GlobalOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { StrategyGroupFilter } from "../../../types/localConfig";
import { COUNTRY_REGION_CODES, countryFlag } from "../countries";
import { lines } from "../resourceModel";
import { insertAtSelection } from "../textInsertion";
import { ResourceFieldLabel } from "./StrategyRuleSetReferencesEditor";

export function NodeNameFilter({
  filter,
  onChange,
}: {
  filter: StrategyGroupFilter;
  onChange: (filter: StrategyGroupFilter) => void;
}) {
  const { t } = useTranslation();
  const [text, setText] = useState(filter.namePatterns.join("\n"));
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  useEffect(() => {
    if (JSON.stringify(lines(text)) !== JSON.stringify(filter.namePatterns))
      setText(filter.namePatterns.join("\n"));
  }, [filter.namePatterns]);
  const input = useRef<TextAreaRef>(null);
  const selection = useRef({ start: text.length, end: text.length });
  const rememberSelection = () => {
    const element = input.current?.resizableTextArea?.textArea;
    if (element)
      selection.current = {
        start: element.selectionStart,
        end: element.selectionEnd,
      };
  };
  const update = (value: string) => {
    setText(value);
    onChange({ ...filter, namePatterns: lines(value) });
  };
  const insert = (code: string) => {
    const next = insertAtSelection(
      text,
      countryFlag(code),
      selection.current.start,
      selection.current.end,
    );
    update(next.text);
    selection.current = { start: next.caret, end: next.caret };
    setOpen(false);
    requestAnimationFrame(() => {
      const element = input.current?.resizableTextArea?.textArea;
      element?.focus();
      element?.setSelectionRange(next.caret, next.caret);
    });
  };
  return (
    <div className="config-resource-field">
      <div className="strategy-rule-set-heading node-name-filter-heading">
        <ResourceFieldLabel
          helpBase="localConfig.resources.groups.help.namePatterns"
          label={t("localConfig.resources.groups.namePatterns")}
        />
        <Popover
          open={open}
          onOpenChange={setOpen}
          trigger="click"
          title={t("localConfig.redesign.insertFlag")}
          content={
            <div className="flag-picker">
              <Input.Search
                aria-label={t("localConfig.redesign.searchFlag")}
                placeholder={t("localConfig.redesign.searchFlag")}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                allowClear
              />
              <div className="flag-picker-grid">
                {COUNTRY_REGION_CODES.filter((code) =>
                  code.includes(query.trim().toUpperCase()),
                ).map((code) => (
                  <Button
                    key={code}
                    onClick={() => insert(code)}
                    aria-label={`${countryFlag(code)} ${code}`}
                  >
                    {countryFlag(code)} {code}
                  </Button>
                ))}
              </div>
            </div>
          }
        >
          <Tooltip title={t("localConfig.redesign.insertFlag")}>
            <Button
              aria-label={t("localConfig.redesign.insertFlag")}
              icon={<GlobalOutlined />}
              onMouseDown={rememberSelection}
            />
          </Tooltip>
        </Popover>
      </div>
      <Input.TextArea
        ref={input}
        aria-label={t("localConfig.resources.groups.namePatterns")}
        autoSize={{ minRows: 6, maxRows: 16 }}
        value={text}
        onChange={(event) => update(event.target.value)}
        onSelect={rememberSelection}
        onBlur={rememberSelection}
        spellCheck={false}
        placeholder={"🇺🇸\n!test"}
      />
    </div>
  );
}
