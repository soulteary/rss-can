if (typeof $ == "undefined") {
  $ = function () {
    return {
      text: function () {
        return "";
      },
      attr: function () {
        return "";
      },
    };
  };
}
