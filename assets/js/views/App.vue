<template>
	<div class="app d-flex flex-column justify-content-between overflow-hidden">
		<div class="flex-grow-1 d-flex flex-column justify-content-between">
			<nav class="navbar navbar-expand-lg navbar-dark bg-dark">
				<div class="container">
					<a class="navbar-brand" href="/" @click.prevent="reload">
						<Logo class="logo me-3"></Logo>
						<span v-if="title" class="title">{{ title }}</span>
					</a>
					<div class="d-flex">
						<div class="d-flex">
							<Notifications :notifications="notifications" />
							<button
								class="navbar-toggler"
								type="button"
								data-bs-toggle="collapse"
								data-bs-target="#navbarNavAltMarkup"
								aria-controls="navbarNavAltMarkup"
								aria-expanded="false"
								aria-label="Toggle navigation"
							>
								<span class="navbar-toggler-icon"></span>
							</button>
						</div>
						<div
							class="collapse navbar-collapse flex-lg-grow-0"
							id="navbarNavAltMarkup"
						>
							<ul class="navbar-nav">
								<li class="nav-item">
									<a
										class="nav-link"
										href="https://github.com/andig/evcc/discussions"
										target="_blank"
									>
										{{ $t("header.support") }}
									</a>
								</li>
							</ul>
						</div>
					</div>
				</div>
			</nav>
			<router-view
				class="flex-grow-1 d-flex flex-column justify-content-stretch"
			></router-view>
		</div>
		<Footer :version="version" :sponsor="sponsor"></Footer>
	</div>
</template>

<script>
import "../icons";
import Logo from "../components/Logo";
import Footer from "../components/Footer";
import Notifications from "../components/Notifications";

import store from "../store";

export default {
	name: "App",
	components: { Logo, Footer, Notifications },
	data: function () {
		return {
			compact: false,
			store: this.$root.$data.store,
			installedVersion: window.evcc.version,
		};
	},
	methods: {
		connect: function () {
			const loc = window.location;
			const protocol = loc.protocol == "https:" ? "wss:" : "ws:";
			const uri =
				protocol +
				"//" +
				loc.hostname +
				(loc.port ? ":" + loc.port : "") +
				loc.pathname +
				"ws";
			const ws = new WebSocket(uri),
				self = this;
			ws.onerror = function () {
				ws.close();
			};
			ws.onclose = function () {
				window.setTimeout(self.connect, 1000);
			};
			ws.onmessage = function (evt) {
				try {
					var msg = JSON.parse(evt.data);
					store.update(msg);
				} catch (e) {
					window.app.error(e, evt.data);
				}
			};
		},
		reload() {
			window.location.reload();
		},
	},
	metaInfo() {
		return {
			title: this.title ? `evcc | ${this.title}` : "evcc",
		};
	},
	computed: {
		version: function () {
			return {
				installed: this.installedVersion,
				available: this.store.state.availableVersion,
				releaseNotes: this.store.state.releaseNotes,
				hasUpdater: this.store.state.hasUpdater,
				uploadMessage: this.store.state.uploadMessage,
				uploadProgress: this.store.state.uploadProgress,
			};
		},
		title: function () {
			return this.store.state.siteTitle;
		},
		sponsor: function () {
			return this.store.state.sponsor;
		},
	},
	props: {
		notifications: Array,
	},
	created: function () {
		const urlParams = new URLSearchParams(window.location.search);
		this.compact = urlParams.get("compact");
		this.connect(); // websocket listener
	},
};
</script>
<style scoped>
.logo {
	width: 85px;
}
.app {
	min-height: 100vh;
}
.title {
	position: relative;
	top: 0.1rem;
}
</style>
