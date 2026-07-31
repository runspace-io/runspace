const menuButton = document.querySelector('[data-menu-button]');
const mobileNav = document.querySelector('[data-mobile-nav]');

const closeMenu = () => {
  menuButton?.setAttribute('aria-expanded', 'false');
  mobileNav?.classList.remove('is-open');
  document.body.classList.remove('menu-open');
};

menuButton?.addEventListener('click', () => {
  const open = menuButton.getAttribute('aria-expanded') !== 'true';
  menuButton.setAttribute('aria-expanded', String(open));
  mobileNav?.classList.toggle('is-open', open);
  document.body.classList.toggle('menu-open', open);
});

mobileNav?.querySelectorAll('a').forEach((link) => link.addEventListener('click', closeMenu));
