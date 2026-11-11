import { Button, Checkbox, Input, Select } from "antd";
import { DeleteOutlined, ReloadOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";
import type { LocalConfigResourceState } from "../../../types/localConfig";
import { flagSelectRenderers } from "../../../components/common/flagSelectRenderers";
import {
  eligibleGroupOptions,
  type ResourceFilters,
  type ResourceKind,
} from "../resourceLibrary";
import { strategyGroupDisplayName } from "../resourceModel";

interface Props {
  kind: ResourceKind;
  state: LocalConfigResourceState;
  filters: ResourceFilters;
  onFilter: (next: ResourceFilters) => void;
  busy: boolean;
  shown: number;
  total: number;
  selected: number;
  selecting: boolean;
  onSelecting: () => void;
  onSelectAll: (checked: boolean) => void;
  onClear: () => void;
  onDelete: () => void;
  onReload: () => void;
  create: ReactNode;
}
export function ResourceLibraryToolbar(props: Props) {
  const { t } = useTranslation();
  const { filters, kind, state, busy, onFilter } = props;
  return (
    <div className="resource-library-controls">
      <div className="resource-library-toolbar">
        <Input
          allowClear
          className="resource-library-search"
          aria-label={t(
            kind === "groups"
              ? "resourceLibrary.searchGroups"
              : "resourceLibrary.searchRules",
          )}
          placeholder={t(
            kind === "groups"
              ? "resourceLibrary.searchGroups"
              : "resourceLibrary.searchRules",
          )}
          value={filters.query}
          onChange={(event) =>
            onFilter({ ...filters, query: event.target.value })
          }
        />
        <Select
          className="resource-library-config-filter"
          aria-label={t("resourceLibrary.config")}
          value={filters.config}
          onChange={(config) => onFilter({ ...filters, config, groups: [] })}
          {...flagSelectRenderers}
          options={[
            { value: "all", label: t("resourceLibrary.allConfigs") },
            { value: "unlinked", label: t("resourceLibrary.noConfig") },
            ...(state.configReferences ?? []).map((item) => ({
              value: item.id,
              label: item.name,
            })),
          ]}
        />
        {kind === "groups" ? (
          <Select
            className="resource-library-kind-filter"
            aria-label={t("resourceLibrary.kind")}
            value={filters.kind}
            onChange={(value) => onFilter({ ...filters, kind: value })}
            options={[
              { value: "all", label: t("resourceLibrary.allKinds") },
              ...["selector", "rule"].map((value) => ({
                value,
                label: t(`localConfig.resources.groups.kinds.${value}`),
              })),
            ]}
          />
        ) : (
          <Select
            className="resource-library-group-filter"
            mode="multiple"
            allowClear
            maxTagCount="responsive"
            aria-label={t("resourceLibrary.groups")}
            placeholder={t("resourceLibrary.allGroups")}
            value={filters.groups}
            onChange={(groups) => onFilter({ ...filters, groups })}
            {...flagSelectRenderers}
            options={[
              { value: "unlinked", label: t("resourceLibrary.noGroup") },
              ...eligibleGroupOptions(state, filters.config).map((item) => ({
                value: item.id,
                label: strategyGroupDisplayName(item),
              })),
            ]}
          />
        )}
        <Button
          disabled={busy}
          aria-pressed={props.selecting}
          onClick={props.onSelecting}
        >
          {t(
            props.selecting ? "resourceLibrary.done" : "resourceLibrary.select",
          )}
        </Button>
        <Button
          disabled={busy}
          icon={<ReloadOutlined />}
          aria-label={t("resourceLibrary.reload")}
          onClick={props.onReload}
        />
        {props.create}
      </div>
      <div className="resource-library-status">
        <span aria-live="polite">
          {t("resourceLibrary.results", {
            shown: props.shown,
            total: props.total,
          })}
        </span>
        {props.selecting ? (
          <div className="resource-library-bulk">
            <Checkbox
              disabled={busy || !props.shown}
              checked={props.shown > 0 && props.selected === props.shown}
              indeterminate={props.selected > 0 && props.selected < props.shown}
              onChange={(event) => props.onSelectAll(event.target.checked)}
            >
              {t("resourceLibrary.selectAll")}
            </Checkbox>
            <span>
              {t("resourceLibrary.selected", { count: props.selected })}
            </span>
            <Button
              size="small"
              type="text"
              disabled={busy || !props.selected}
              onClick={props.onClear}
            >
              {t("resourceLibrary.clear")}
            </Button>
            <Button
              size="small"
              danger
              icon={<DeleteOutlined />}
              disabled={busy || !props.selected}
              onClick={props.onDelete}
            >
              {t("resourceLibrary.delete")}
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
