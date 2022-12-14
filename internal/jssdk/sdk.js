class POTTED {
  constructor($scope, $config) {
    if ($scope == "") {
      this.scope = $("");
      this.container = "";
    } else {
      this.scope = $($scope);
      this.container = $scope;
    }
    if ($config) {
      this.config = $config;
      this.isConfigured = true;
    } else {
      this.isConfigured = false;
      this.config = {};
    }
    this.timestamp = new Date() - 0;
    this.value = null;
  }

  // manually set the scope
  SetScope($selector) {
    this.scope = $($selector);
    this.container = $selector;
    return this;
  }

  // set rule configuration
  SetConfig($config) {
    this.config = $config;
    // TODO: check properties exists
    // TODO: check properties value not empty
    this.isConfigured = true;
    return this;
  }

  // get configuration
  GetConfig() {
    if (this.isConfigured) {
      this.config.ListContainer = this.container;
      return this.config;
    }
    return {};
  }

  // expose static utility methods
  static fn = {
    $: $,
  };

  // expose method to get DOM text
  static Text($selector) {
    return $($selector).text().trim();
  }

  // get text from selector
  GetText($selector, $parentSelector) {
    let scope = this.scope;
    if ($parentSelector != "") scope = $($parentSelector);
    return scope.find($selector).text().trim();
  }

  // expose method to get DOM link address
  static Link($selector) {
    return $($selector).attr("href").trim();
  }

  // get the link address according to the selector
  GetLink($selector, $parentSelector) {
    let scope = this.scope;
    if ($parentSelector != "") scope = $($parentSelector);
    return scope.find($selector).attr("href").trim();
  }

  // get data according to configuration
  GetData($config) {
    if (this.scope.length == 0) return [];
    if ($config) this.SetConfig($config);
    if (this.isConfigured == false) return [];

    let result = [];
    let self = this;
    this.scope.each((_, el) => {
      const title = self.GetText(self.config.Title, el);
      const author = self.GetText(self.config.Author, el);
      const date = self.GetText(self.config.DateTime, el);
      const description = self.GetText(self.config.Description, el);
      const link = self.GetLink(self.config.Link, el);

      result.push({ title, author, category, date, description, link });
    });
    this.value = result;
    return this;
  }

  PostScript() {
    // TODO: add post script hook
  }

  valueOf() {
    return this.value;
  }
}
