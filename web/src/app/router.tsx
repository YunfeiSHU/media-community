import { lazy, Suspense } from 'react';
import { Route, Routes } from 'react-router-dom';
import { RequireAuth } from '@/app/guards';
import { SiteLayout } from '@/app/layouts/SiteLayout';
import LoginPage from '@/pages/auth/LoginPage';
import RegisterPage from '@/pages/auth/RegisterPage';
import HomePage from '@/pages/home/HomePage';
import LiveListPage from '@/pages/live/LiveListPage';
import LiveManagePage from '@/pages/live/LiveManagePage';
import LiveRoomPage from '@/pages/live/LiveRoomPage';
import OrderDetailPage from '@/pages/orders/OrderDetailPage';
import OrdersPage from '@/pages/orders/OrdersPage';
import SearchPage from '@/pages/search/SearchPage';
import ProductPage from '@/pages/shop/ProductPage';
import ShopPage from '@/pages/shop/ShopPage';
import SpacePage from '@/pages/space/SpacePage';
import ProfileSettingsPage from '@/pages/user/ProfileSettingsPage';
import UserProfilePage from '@/pages/user/UserProfilePage';
import VideoManagePage from '@/pages/studio/VideoManagePage';
import ProductManagePage from '@/pages/studio/ProductManagePage';
import AdminPage from '@/pages/admin/AdminPage';
import UploadPage from '@/pages/upload/UploadPage';
import { NotFoundPage } from '@/pages/not-found/NotFoundPage';
import { CardGridSkeleton } from '@/widgets/empty-state/EmptyState';

// 播放页会把 hls.js 一起加载，单独拆包，避免拖累首页首屏
const VideoPage = lazy(() => import('@/pages/video/VideoPage'));

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<SiteLayout />}>
        <Route index element={<HomePage />} />
        <Route path="search" element={<SearchPage />} />
        <Route
          path="video/:id"
          element={
            <Suspense fallback={<CardGridSkeleton count={2} />}>
              <VideoPage />
            </Suspense>
          }
        />
        {/* 前端直播路由用 /streams：/live 前缀被媒体反代（SRS）占用，
            放在 /live 下会先命中代理而不是 SPA（见《开发问题记录》I-011） */}
        <Route path="streams" element={<LiveListPage />} />
        <Route
          path="streams/manage"
          element={
            <RequireAuth>
              <LiveManagePage />
            </RequireAuth>
          }
        />
        <Route path="streams/:id" element={<LiveRoomPage />} />
        <Route path="shop" element={<ShopPage />} />
        <Route path="shop/:id" element={<ProductPage />} />
        <Route
          path="orders"
          element={
            <RequireAuth>
              <OrdersPage />
            </RequireAuth>
          }
        />
        <Route
          path="orders/:id"
          element={
            <RequireAuth>
              <OrderDetailPage />
            </RequireAuth>
          }
        />
        <Route path="login" element={<LoginPage />} />
        <Route path="register" element={<RegisterPage />} />
        <Route
          path="upload"
          element={
            <RequireAuth>
              <UploadPage />
            </RequireAuth>
          }
        />
        <Route
          path="space"
          element={
            <RequireAuth>
              <SpacePage />
            </RequireAuth>
          }
        />
        <Route path="user/:uid" element={<UserProfilePage />} />
        <Route
          path="settings"
          element={
            <RequireAuth>
              <ProfileSettingsPage />
            </RequireAuth>
          }
        />
        <Route
          path="studio/videos"
          element={
            <RequireAuth>
              <VideoManagePage />
            </RequireAuth>
          }
        />
        <Route
          path="studio/products"
          element={
            <RequireAuth>
              <ProductManagePage />
            </RequireAuth>
          }
        />
        <Route
          path="admin"
          element={
            <RequireAuth>
              <AdminPage />
            </RequireAuth>
          }
        />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}



