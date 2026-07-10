<script setup>
import "leaflet/dist/leaflet.css";
import {
  LCircle,
  LIcon,
  LMap,
  LMarker,
  LPopup,
  LTileLayer,
  LTooltip,
} from "@vue-leaflet/vue-leaflet";
import { AddCircle20Filled, SubtractCircle20Filled } from "@vicons/fluent";
import ApiService from "@/service/api.js";
import IconBase from "@/assets/map/base.webp";
import IconPlayer from "@/assets/map/player.webp";
import IconBossTower from "@/assets/map/boss_tower.webp";
import IconFastTravel from "@/assets/map/fast_travel.webp";
import playerToGuildStore from "@/stores/model/playerToGuild.js";
import points from "@/assets/map/points.json";

const LAND_SCAPE = [447900, 708920, -999940, -738920];

const api = new ApiService();

const mousePosition = ref([0, 0]);
const zoom = ref(2);
const tiles = ref("map/tiles/{z}/{x}/{y}.png");
const playerList = ref([]);
const guildList = ref([]);
const showPlayer = ref(true);
const showBaseCamp = ref(true);
const showBossTower = ref(false);
const showFastTravel = ref(false);

let timer = null;

const toMapPosition = (position) => {
  // hack
  if (position[0] >= -256 && position[0] <= 256) {
    return position;
  }
  const x =
    -256 +
    (256 * (position[0] - LAND_SCAPE[2])) / (LAND_SCAPE[0] - LAND_SCAPE[2]);
  const y =
    (256 * (position[1] - LAND_SCAPE[3])) / (LAND_SCAPE[1] - LAND_SCAPE[3]);
  return [x, y];
};

const toMapDistance = (distance) => {
  return 256 * (distance / (LAND_SCAPE[0] - LAND_SCAPE[2]));
};

const ToPlayers = async (uid) => {
  playerToGuildStore().setCurrentUid(uid);
  playerToGuildStore().setUpdateStatus("players");
};

const refreshPlayer = async () => {
  const { data } = await api.getOnlinePlayerList();
  for (const i of data.value) {
    for (const j of playerList.value) {
      if (i.player_uid === j.player_uid) {
        j.location_x = i.location_x;
        j.location_y = i.location_y;
        break;
      }
    }
  }
  timer = setTimeout(refreshPlayer, 5000);
};

const onMapMouseMove = (event) => {
  mousePosition.value = [
    event.latlng.lat.toFixed(2),
    event.latlng.lng.toFixed(2),
  ];
};

// 左下角控件
const onAddZoom = () => {
  if (zoom.value !== 6) {
    zoom.value += 1;
  }
};
const onSubtractZoom = () => {
  if (zoom.value !== 0) {
    zoom.value -= 1;
  }
};

onMounted(async () => {
  let res = await api.getPlayerList({});
  playerList.value = res.data.value;
  // 接口中玩家location_x和location_y同时为0时，表示玩家不在线，不显示
  playerList.value = playerList.value.filter(
    (i) => i.location_x !== 0 && i.location_y !== 0,
  );
  res = await api.getGuildList();
  guildList.value = res.data.value;

  refreshPlayer();
});

onUnmounted(async () => {
  clearTimeout(timer);
});
</script>

<template>
  <div class="map-view h-full">
    <l-map
      ref="map"
      style="width: 100%; height: 100%"
      crs="Simple"
      v-model:zoom="zoom"
      :use-global-leaflet="false"
      :center="[-128, 128]"
      :min-zoom="0"
      :max-zoom="6"
      :options="{ zoomControl: false, attributionControl: false }"
      @mousemove="onMapMouseMove"
    >
      <l-tile-layer
        :url="tiles"
        no-wrap
        layer-type="base"
        :options="{
          bounds: [
            [0, 0],
            [-256, 256],
          ],
        }"
      ></l-tile-layer>
      <template v-if="showFastTravel">
        <l-marker
          v-for="(point, index) in points.fast_travel"
          :key="`fast-travel-${index}-${point[0]}-${point[1]}`"
          :lat-lng="toMapPosition([point[0], point[1]])"
        >
          <l-icon :icon-url="IconFastTravel" :icon-size="[48, 48]" />
        </l-marker>
      </template>
      <template v-if="showBossTower">
        <l-marker
          v-for="(point, index) in points.boss_tower"
          :key="`boss-tower-${index}-${point[0]}-${point[1]}`"
          :lat-lng="toMapPosition([point[0], point[1]])"
        >
          <l-icon :icon-url="IconBossTower" :icon-size="[48, 48]" />
        </l-marker>
      </template>
      <template v-if="showPlayer">
        <l-marker
          v-for="(player, index) in playerList"
          :key="player.user_id || player.player_uid || `player-${index}`"
          :lat-lng="toMapPosition([player.location_x, player.location_y])"
        >
          <l-icon :icon-url="IconPlayer" :icon-size="[45, 45]" />
          <l-tooltip
            :options="{ direction: 'top', permanent: true, offset: [0, -15] }"
            >{{ player.nickname }}</l-tooltip
          >
        </l-marker>
      </template>
      <template v-if="showBaseCamp">
        <template v-for="guild in guildList" :key="guild.admin_player_uid">
          <template v-for="camp in guild.base_camp" :key="camp.id">
            <l-marker
              :lat-lng="toMapPosition([camp.location_x, camp.location_y])"
            >
              <l-icon :icon-url="IconBase" :icon-size="[55, 55]" />
              <l-popup :options="{ interactive: true }">
                <div style="padding-bottom: 3px; font-size: 16px">
                  {{ $t("map.baseCampTitle", { name: guild.name }) }}
                </div>
                <div style="line-height: 25px">
                  {{ $t("map.guildMember") }}
                  <span
                    v-for="player in guild.players"
                    :key="player.player_uid"
                    class="player_name"
                    @click="ToPlayers(player.player_uid)"
                  >
                    {{ player.nickname }}
                  </span>
                </div>
              </l-popup>
            </l-marker>
            <l-circle
              :lat-lng="toMapPosition([camp.location_x, camp.location_y])"
              :radius="toMapDistance(camp.area)"
            />
          </template>
        </template>
      </template>
    </l-map>
    <div
      class="min-h-50 p-2 fixed bottom-2 left-2 z-999 flex flex-col justify-end"
    >
      <div class="h-40 flex flex-col justify-between items-center">
        <n-icon
          class="cursor-pointer"
          size="24"
          color="#fff"
          @click="onAddZoom"
        >
          <AddCircle20Filled />
        </n-icon>
        <n-slider
          style="height: 100px"
          class="border border-solid border-#fff rounded-full"
          v-model:value="zoom"
          :tooltip="false"
          :height="4"
          :step="1"
          :min="0"
          :max="6"
          vertical
        />
        <n-icon
          class="cursor-pointer"
          size="24"
          color="#fff"
          @click="onSubtractZoom"
        >
          <SubtractCircle20Filled />
        </n-icon>
      </div>
    </div>
    <div class="control">
      <div>
        <span>{{ $t("map.showFastTravel") }}</span>
        <n-switch v-model:value="showFastTravel" />
      </div>
      <div>
        <span>{{ $t("map.showBossTower") }}</span>
        <n-switch v-model:value="showBossTower" />
      </div>
      <div>
        <span>{{ $t("map.showPlayer") }}</span>
        <n-switch v-model:value="showPlayer" />
      </div>
      <div>
        <span>{{ $t("map.showBaseCamp") }}</span>
        <n-switch v-model:value="showBaseCamp" />
      </div>
      <div>
        <span>{{ mousePosition[0] }}, {{ mousePosition[1] }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.leaflet-container {
  background: #102536;
  outline: 0;
}

.player_name {
  margin: 0 3px;
  padding: 3px;
  color: #fff;
  background-color: #009f5d;
  border-radius: 3px;
}

.control {
  width: 200px;
  height: 180px;
  position: absolute;
  bottom: 20px;
  right: 20px;
  color: #fff;
  background-color: rgb(24, 24, 28);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  justify-content: space-around;
  z-index: 999;
}

.control > div {
  display: flex;
  justify-content: space-between;
  margin: 10px;
}
</style>
