import "bootstrap/dist/css/bootstrap.min.css";
import "bootstrap";
import "../css/app.css";
import Vue from "vue";
import VueMeta from "vue-meta";
import axios from "axios";
import App from "./views/App";
import router from "./router";
import i18n from "./i18n";
import store from "./store";

Vue.use(VueMeta);

const loc = window.location;
axios.defaults.baseURL =
  loc.protocol + "//" + loc.hostname + (loc.port ? ":" + loc.port : "") + loc.pathname + "api";
axios.defaults.headers.post["Content-Type"] = "application/json";

window.app = new Vue({
  el: "#app",
  router,
  i18n,
  data: { store, notifications: [] },
  render: function (h) {
    return h(App, { props: { notifications: this.notifications } });
  },
  methods: {
    raise: function (msg) {
      console[msg.type](msg);
      const now = new Date();
      const latestMsg = this.notifications[0];
      if (latestMsg && latestMsg.message === msg.message) {
        latestMsg.count++;
        latestMsg.time = now;
      } else {
        this.notifications = [
          {
            ...msg,
            count: 1,
            time: now,
          },
          ...this.notifications,
        ];
      }
    },
    clear: function () {
      this.notifications = [];
    },
    error: function (msg) {
      msg.type = "error";
      this.raise(msg);
    },
    warn: function (msg) {
      msg.type = "warn";
      this.raise(msg);
    },
  },
});

window.setInterval(function () {
  axios.get("health").catch(function () {
    window.app.error({ message: "Server unavailable" });
  });
}, 5000);
