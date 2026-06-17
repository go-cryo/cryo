import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('pages/LoginPage.vue'),
  },
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', name: 'home', component: () => import('pages/IndexPage.vue') },
      { path: 'hosts', name: 'hosts', component: () => import('pages/hosts/HostListPage.vue') },
      { path: 'hosts/add', name: 'host-add', component: () => import('pages/hosts/HostAddPage.vue') },
      { path: 'hosts/:namespace/:name/edit', name: 'host-edit', component: () => import('pages/hosts/HostEditPage.vue') },
      { path: 'hosts/:namespace/:name', name: 'host-detail', component: () => import('pages/hosts/HostDetailPage.vue') },
      { path: 'backupjobs', name: 'backupjobs', component: () => import('pages/backupjobs/BackupJobListPage.vue') },
      { path: 'backupjobs/add', name: 'backupjob-add', component: () => import('pages/backupjobs/BackupJobAddPage.vue') },
      { path: 'backupjobs/:namespace/:name/edit', name: 'backupjob-edit', component: () => import('pages/backupjobs/BackupJobEditPage.vue') },
      { path: 'backupjobs/:namespace/:name', name: 'backupjob-detail', component: () => import('pages/backupjobs/BackupJobDetailPage.vue') },
      { path: 'repositories', name: 'repositories', component: () => import('pages/repositories/RepositoryListPage.vue') },
      { path: 'repositories/add', name: 'repository-add', component: () => import('pages/repositories/RepositoryAddPage.vue') },
      { path: 'repositories/:namespace/:name/edit', name: 'repository-edit', component: () => import('pages/repositories/RepositoryEditPage.vue') },
      { path: 'repositories/:namespace/:name/snapshots/:snapshotId', name: 'snapshot-browse', component: () => import('pages/repositories/SnapshotBrowsePage.vue') },
      { path: 'repositories/:namespace/:name', name: 'repository-detail', component: () => import('pages/repositories/RepositoryDetailPage.vue') },
      { path: 'settings', name: 'settings', component: () => import('pages/SettingsPage.vue') },
    ],
  },

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue'),
  },
];

export default routes;
