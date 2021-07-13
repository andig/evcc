<template>
	<div class="visualization" :class="{ 'visualization--ready': totalAdjusted > 0 }">
		<div class="label-scale">
			<div class="d-flex justify-content-end">
				<div
					class="label-bar label-bar--down"
					:class="{
						'label-bar--invisible':
							hideLabelBar(batteryDischarge) || !selfConsumptionAdjusted,
					}"
					:style="{ width: widthTotal(batteryDischarge) }"
				>
					<div class="label-bar-scale">
						<div class="label-bar-icon">
							<fa-icon :icon="batteryIcon"></fa-icon>
							<fa-icon icon="caret-right"></fa-icon>
						</div>
					</div>
				</div>
				<div
					class="label-bar label-bar--down"
					:class="{
						'label-bar--invisible':
							hideLabelBar(pvProduction) ||
							(!selfConsumptionAdjusted && !pvExportAdjusted),
					}"
					:style="{ width: widthTotal(pvProduction) }"
				>
					<div class="label-bar-scale">
						<div class="label-bar-icon">
							<fa-icon icon="sun"></fa-icon>
						</div>
					</div>
				</div>
			</div>
		</div>
		<div class="site-progress" ref="site_progress">
			<div class="site-progress-bar grid-import" :style="{ width: widthTotal(gridImport) }">
				<span class="power" :class="{ 'd-none': hidePowerLabel(gridImport) }">
					{{ kw(gridImport) }}
				</span>
			</div>
			<div
				class="site-progress-bar self-consumption"
				:style="{ width: widthTotal(selfConsumptionAdjusted) }"
			>
				<span
					class="power"
					:class="{
						'd-none': hidePowerLabel(selfConsumption),
					}"
				>
					{{ kw(selfConsumption) }}
				</span>
			</div>
			<div
				class="site-progress-bar pv-export"
				:style="{ width: widthTotal(pvExportAdjusted) }"
			>
				<span class="power" :class="{ 'd-none': hidePowerLabel(pvExport) }">
					{{ kw(pvExport) }}
				</span>
			</div>
		</div>
		<div class="label-scale">
			<div class="d-flex justify-content-start">
				<div
					class="label-bar label-bar--up"
					:class="{
						'label-bar--invisible':
							hideLabelBar(houseConsumption) ||
							(!gridImportAdjusted && !selfConsumptionAdjusted),
					}"
					:style="{ width: widthTotal(houseConsumption) }"
				>
					<div class="label-bar-scale">
						<div class="label-bar-icon">
							<fa-icon icon="home"></fa-icon>
						</div>
					</div>
				</div>
				<div
					class="label-bar label-bar--up"
					:class="{
						'label-bar--invisible':
							hideLabelBar(batteryCharge) || !selfConsumptionAdjusted,
					}"
					:style="{ width: widthTotal(batteryCharge) }"
				>
					<div class="label-bar-scale">
						<div class="label-bar-icon">
							<fa-icon :icon="batteryIcon"></fa-icon>
							<fa-icon icon="caret-left"></fa-icon>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
import "../../icons";
import formatter from "../../mixins/formatter";

export default {
	name: "Visualization",
	props: {
		showDetails: Boolean,
		gridImport: { type: Number, default: 0 },
		selfConsumption: { type: Number, default: 0 },
		pvExport: { type: Number, default: 0 },
		batteryCharge: { type: Number, default: 0 },
		batteryDischarge: { type: Number, default: 0 },
		pvProduction: { type: Number, default: 0 },
		houseConsumption: { type: Number, default: 0 },
		batteryIcon: { type: String },
	},
	data: function () {
		return { width: 0 };
	},
	mounted: function () {
		this.$nextTick(function () {
			window.addEventListener("resize", this.updateElementWidth);
			this.updateElementWidth();
		});
	},
	beforeDestroy() {
		window.removeEventListener("resize", this.updateElementWidth);
	},
	mixins: [formatter],
	computed: {
		totalRaw: function () {
			return this.gridImport + this.selfConsumption + this.pvExport;
		},
		gridImportAdjusted: function () {
			return this.applyThreshold(this.gridImport);
		},
		selfConsumptionAdjusted: function () {
			return this.applyThreshold(this.selfConsumption);
		},
		pvExportAdjusted: function () {
			return this.applyThreshold(this.pvExport);
		},
		totalAdjusted: function () {
			return this.gridImportAdjusted + this.selfConsumptionAdjusted + this.pvExportAdjusted;
		},
	},
	watch: {
		showDetails: function () {
			this.$nextTick(() => this.updateElementWidth());
		},
	},
	methods: {
		widthTotal: function (power) {
			return (100 / this.totalAdjusted) * power + "%";
		},
		kw: function (watt) {
			return Math.max(0, watt / 1000).toFixed(1) + " kW";
		},
		hidePowerLabel(power) {
			const minWidth = 75;
			const percent = (100 / this.totalAdjusted) * power;
			return (this.width / 100) * percent < minWidth;
		},
		hideLabelBar(power) {
			const minWidth = 60;
			const percent = (100 / this.totalAdjusted) * power;
			return (this.width / 100) * percent < minWidth;
		},
		applyThreshold(power) {
			const percent = (100 / this.totalRaw) * power;
			return percent < 2 ? 0 : power;
		},
		updateElementWidth() {
			this.width = this.$refs.site_progress.getBoundingClientRect().width;
		},
	},
};
</script>
<style scoped>
.visualization {
	opacity: 0;
}
.visualization--ready {
	opacity: 1;
}
.site-progress {
	margin: 0.25rem 0;
	border-radius: 5px;
	display: flex;
	overflow: hidden;
}
.site-progress-bar {
	display: flex;
	transition-property: width;
	transition-duration: 1000ms;
	transition-timing-function: linear;
	justify-content: center;
	align-items: center;
	overflow: hidden;
	height: 1.5rem;
	position: relative;
}
.site-progress-bar::before,
.site-progress-bar::after {
	content: "";
	top: 0;
	bottom: 0;
	width: 1px;
	background: white;
	position: absolute;
}
.site-progress-bar::before {
	left: 0px;
}
.site-progress-bar::after {
	right: 0px;
}
.grid-import {
	background-color: var(--evcc-grid);
	color: var(--bs-white);
}
.self-consumption {
	background-color: var(--evcc-self);
	color: var(--bs-white);
}
.pv-export {
	background-color: var(--evcc-export);
	color: var(--bs-dark);
}
.power {
	display: block;
	margin: 0 0.5rem;
	white-space: nowrap;
}
.label-bar {
	margin: 0;
	height: 1.5rem;
	padding: 0.5rem 0;
	opacity: 1;
	transition-property: width, opacity;
	transition-duration: 1000ms, 2000ms;
	transition-timing-function: linear, ease-in-out;
	overflow: hidden;
}
.label-bar-scale {
	border: 1px solid var(--bs-gray);
	height: 7px;
	background: none;
	display: flex;
	justify-content: center;
	align-items: center;
	white-space: nowrap;
}
.label-bar--down .label-bar-scale {
	border-bottom: 5px solid transparent;
}
.label-bar--up .label-bar-scale {
	border-top: 5px solid transparent;
}
.label-bar-icon {
	background-color: white;
	color: var(--bs-gray);
	padding: 0 0.75rem;
}
.label-bar--invisible {
	opacity: 0;
}
</style>
