// FieldDOMTags is injected from Go so both bridges recognize the same bare tags.
function RSSCanFieldExpression(input) {
  const field = String(input == null ? "" : input).trim();
  if (field.startsWith("literal:")) return {value: field.slice(8), selector: false, explicit: true};
  if (field.startsWith("css:")) return {value: field.slice(4).trim(), selector: true, explicit: true};
  const selector = field !== "" && (/[.#\[\]:>+~,* \t\r\n\f]/.test(field) || RSS_CAN_FIELD_TAGS.has(field.toLowerCase()));
  return {value: field, selector, explicit: false};
}

function RSSCanReadField(input, method, scope) {
  const expression = RSSCanFieldExpression(input);
  // The empty expression preserves current-item attribute lookup for hooks.
  if (!expression.explicit && expression.value === "") {
    const value = scope.attr(method);
    return value == null ? "" : String(value).trim();
  }
  if (!expression.selector) return expression.value;
  if (!expression.value) return "";
  const selection = scope.find(expression.value);
  if (method === "text") return selection.text().trim();
  if (method === "html") return selection.html() || "";
  const value = selection.first().attr(method);
  return value == null ? "" : String(value).trim();
}
