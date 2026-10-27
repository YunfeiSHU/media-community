import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/features/auth/store';
import { updateMe } from '@/entities/user/api';
import { uploadImage } from '@/entities/upload/api';
import { describeError } from '@/shared/api/errors';
import { Avatar } from '@/shared/ui/Avatar';

/** 个人资料编辑：昵称 / 头像 / 性别 / 简介。 */
export default function ProfileSettingsPage() {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const refreshUser = useAuthStore((state) => state.refreshUser);

  const [nickname, setNickname] = useState(user?.nickname ?? '');
  const [bio, setBio] = useState(user?.bio ?? '');
  const [gender, setGender] = useState(user?.gender || 'unknown');
  const [avatar, setAvatar] = useState(user?.avatar ?? '');
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function handleAvatar(file: File | null) {
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      const image = await uploadImage(file, 'avatar');
      setAvatar(image.url);
      setNotice('头像已上传，保存后生效');
    } catch (uploadError) {
      setError(describeError(uploadError));
    } finally {
      setUploading(false);
    }
  }

  async function save() {
    if (!nickname.trim()) {
      setError('昵称不能为空');
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      await updateMe({
        nickname: nickname.trim(),
        bio: bio.trim(),
        gender,
        avatar,
      });
      await refreshUser();
      setNotice('已保存');
      navigate(`/user/${user?.uid ?? ''}`);
    } catch (saveError) {
      setError(describeError(saveError));
    } finally {
      setSaving(false);
    }
  }

  if (!user) {
    return <p className="text-sm text-ink-tertiary">资料加载中…</p>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-5">
      <div className="flex items-center justify-between">
        <h1 className="text-h1 font-semibold text-ink">编辑资料</h1>
        <Link to={`/user/${user.uid}`} className="text-meta text-ink-tertiary hover:text-brand">
          查看我的主页 →
        </Link>
      </div>

      <section className="ui-card space-y-4 p-5">
        <div className="flex items-center gap-4">
          <Avatar src={avatar} alt="头像" className="h-16 w-16 shrink-0" />
          <div className="space-y-1">
            <label className="ui-btn-ghost h-8 cursor-pointer px-3 text-xs">
              {uploading ? '上传中…' : '上传头像'}
              <input
                type="file"
                accept="image/*"
                className="hidden"
                disabled={uploading}
                onChange={(event) => void handleAvatar(event.target.files?.[0] ?? null)}
              />
            </label>
            <p className="text-meta text-ink-tertiary">建议正方形图片，≤ 5MB</p>
            {avatar && (
              <button
                type="button"
                onClick={() => setAvatar('')}
                className="text-meta text-ink-tertiary hover:text-brand"
              >
                清除头像
              </button>
            )}
          </div>
        </div>

        <label className="block text-meta text-ink-tertiary">
          昵称
          <input
            value={nickname}
            onChange={(event) => setNickname(event.target.value)}
            maxLength={64}
            className="ui-input mt-1.5"
          />
        </label>

        <label className="block text-meta text-ink-tertiary">
          性别
          <select
            value={gender}
            onChange={(event) => setGender(event.target.value)}
            className="ui-input mt-1.5"
          >
            <option value="unknown">保密</option>
            <option value="male">男</option>
            <option value="female">女</option>
          </select>
        </label>

        <label className="block text-meta text-ink-tertiary">
          简介
          <textarea
            value={bio}
            onChange={(event) => setBio(event.target.value)}
            maxLength={500}
            rows={4}
            placeholder="介绍一下自己…"
            className="mt-1.5 w-full rounded-sm border border-line bg-subtle px-3 py-2 text-sm text-ink placeholder:text-ink-tertiary focus:border-brand focus:bg-card focus:outline-none"
          />
        </label>

        {error && <p className="text-meta text-danger">{error}</p>}
        {notice && <p className="text-meta text-ink-tertiary">{notice}</p>}

        <div className="flex gap-2">
          <button type="button" onClick={() => void save()} disabled={saving} className="ui-btn-primary">
            {saving ? '保存中…' : '保存'}
          </button>
          <Link to="/space" className="ui-btn-ghost">
            返回空间
          </Link>
        </div>
      </section>
    </div>
  );
}
