<template>
  <q-page padding>
    <div class="row items-center q-mb-lg">
      <div class="text-h5 text-weight-medium">Repositories</div>
      <q-space />
      <q-btn
        color="primary"
        icon="add"
        label="Add Repository"
        no-caps
        unelevated
        :to="{ name: 'repository-add' }"
      />
    </div>

    <q-card v-if="loading" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-spinner-dots color="primary" size="40px" />
        <div class="text-grey q-mt-sm">Loading repositories...</div>
      </q-card-section>
    </q-card>

    <q-card v-else-if="error" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-icon name="error_outline" color="negative" size="48px" />
        <div class="text-negative q-mt-sm">{{ error }}</div>
        <q-btn
          class="q-mt-md"
          color="primary"
          label="Retry"
          no-caps
          outline
          @click="fetchRepositories"
        />
      </q-card-section>
    </q-card>

    <q-card v-else-if="repositories.length === 0" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-icon name="inventory_2" color="grey-6" size="64px" />
        <div class="text-h6 text-grey-5 q-mt-md">No repositories configured</div>
        <div class="text-grey q-mt-xs">Add a restic repository to get started.</div>
        <q-btn
          class="q-mt-lg"
          color="primary"
          icon="add"
          label="Add Repository"
          no-caps
          unelevated
          :to="{ name: 'repository-add' }"
        />
      </q-card-section>
    </q-card>

    <div v-else class="row q-col-gutter-md">
      <div
        v-for="repo in repositories"
        :key="`${repo.namespace}/${repo.name}`"
        class="col-12 col-sm-6 col-lg-4"
      >
        <q-card
          flat
          bordered
          class="cryo-card cryo-card--clickable cursor-pointer"
          @click="goToDetail(repo)"
        >
          <q-card-section>
            <div class="row items-center no-wrap q-mb-sm">
              <q-icon :name="repoIcon(repo.type)" color="primary" size="24px" class="q-mr-sm" />
              <div class="text-subtitle1 text-weight-medium ellipsis">{{ repo.name }}</div>
            </div>
            <div class="row items-center q-gutter-x-sm">
              <q-badge :color="typeColor(repo.type)" :label="repo.type" />
              <span class="text-caption text-grey">{{ repo.namespace }}</span>
            </div>
            <div class="text-caption text-grey-6 q-mt-sm ellipsis">{{ repo.url }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import repositoriesApi, { type Repository, type RepositoryType } from 'src/api/repositories';

const router = useRouter();
const repositories = ref<Repository[]>([]);
const loading = ref(true);
const error = ref('');

function repoIcon(type: RepositoryType): string {
  switch (type) {
    case 's3': return 'cloud';
    case 'sftp': return 'dns';
    case 'rest': return 'language';
    case 'local': return 'folder';
    default: return 'inventory_2';
  }
}

function typeColor(type: RepositoryType): string {
  switch (type) {
    case 's3': return 'orange';
    case 'sftp': return 'purple';
    case 'rest': return 'blue';
    case 'local': return 'green';
    default: return 'grey';
  }
}

function goToDetail(repo: Repository) {
  router.push({ name: 'repository-detail', params: { namespace: repo.namespace, name: repo.name } });
}

async function fetchRepositories() {
  loading.value = true;
  error.value = '';
  try {
    repositories.value = await repositoriesApi.listRepositories();
  } catch (e) {
    error.value = 'Failed to load repositories.';
    console.error('Failed to fetch repositories:', e);
  } finally {
    loading.value = false;
  }
}

onMounted(fetchRepositories);
</script>

