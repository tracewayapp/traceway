import type { RawThemeSetting, ThemeRegistrationRaw } from "shiki";

const tokenColors: RawThemeSetting[] = [
  {
    settings: { foreground: "#c9d0dd", background: "#00000000" },
  },
  {
      scope: ["comment", "punctuation.definition.comment", "string.comment"],
      settings: { foreground: "#5a6374", fontStyle: "italic" },
    },
    {
      scope: [
        "keyword",
        "storage.type",
        "storage.modifier",
        "keyword.control",
        "keyword.operator.new",
        "keyword.operator.expression",
      ],
      settings: { foreground: "#7c5cff" },
    },
    {
      scope: [
        "keyword.operator",
        "punctuation.separator",
        "punctuation.terminator",
        "punctuation.accessor",
        "meta.brace",
        "punctuation.definition.parameters",
      ],
      settings: { foreground: "#8a93a6" },
    },
    {
      scope: [
        "string",
        "string.quoted",
        "string.template",
        "punctuation.definition.string",
      ],
      settings: { foreground: "#22e0a8" },
    },
    {
      scope: [
        "constant.numeric",
        "constant.language",
        "constant.character",
        "constant.other",
        "support.constant",
      ],
      settings: { foreground: "#ffd166" },
    },
    {
      scope: [
        "entity.name.function",
        "support.function",
        "meta.function-call.generic",
        "variable.function",
      ],
      settings: { foreground: "#00d4ff" },
    },
    {
      scope: [
        "entity.name.type",
        "entity.name.class",
        "support.type",
        "support.class",
        "entity.other.inherited-class",
        "storage.type.class",
      ],
      settings: { foreground: "#00d4ff" },
    },
    {
      scope: [
        "variable",
        "variable.other",
        "variable.parameter",
        "meta.definition.variable",
      ],
      settings: { foreground: "#c9d0dd" },
    },
    {
      scope: [
        "variable.other.property",
        "meta.object-literal.key",
        "support.type.property-name",
      ],
      settings: { foreground: "#c9d0dd" },
    },
    {
      scope: ["support.type.property-name.json"],
      settings: { foreground: "#00d4ff" },
    },
    {
      scope: ["meta.tag", "entity.name.tag", "punctuation.definition.tag"],
      settings: { foreground: "#7c5cff" },
    },
    {
      scope: ["entity.other.attribute-name", "entity.name.label"],
      settings: { foreground: "#ffd166" },
    },
    {
      scope: ["invalid", "invalid.illegal", "message.error"],
      settings: { foreground: "#ff5a5f" },
    },
    {
      scope: [
        "keyword.other.import",
        "keyword.control.import",
        "keyword.control.export",
        "keyword.control.flow",
      ],
      settings: { foreground: "#7c5cff" },
    },
    {
      scope: [
        "entity.name.namespace",
        "entity.name.type.class.dart",
        "storage.type.annotation.dart",
      ],
      settings: { foreground: "#00d4ff" },
    },
];

export const tracewayShiki: ThemeRegistrationRaw = {
  name: "traceway-dark",
  type: "dark",
  colors: {
    "editor.background": "#00000000",
    "editor.foreground": "#c9d0dd",
  },
  settings: tokenColors,
  tokenColors,
};
