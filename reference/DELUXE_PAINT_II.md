# Référence Deluxe Paint II pour Pixeluxe

## Sources et extraction

Référence principale : le disque fourni dans `previous/Deluxe_Paint_II_1987_Electronic_Arts_PAL.adf`. Image de 901 120 octets, système de fichiers Amiga OFS `DOS/0`, volume `DPaint`, racine au bloc 880. SHA-256 : `a519c2be8d89765d7198b908bdc7797f9c00071d4dedfdc03db31df4b50ff9a3`.

Le programme extrait `adf/dpaint` contient l'identification « Deluxe Paint - Version 2.0P », copyright 1986–1987 Daniel Silva et Electronic Arts. Il mesure 191 428 octets ; SHA-256 `b775e59d9fdf9c6038be3383f10781b4b065b3d4b301d7907629781bb4b64581`. La chaîne `Release 2.48` existe également, mais ne doit pas être confondue avec le numéro affiché du logiciel.

Extraction reproductible avec le programme Go standard du dépôt :

```sh
go run cmd/adfextract/main.go previous/Deluxe_Paint_II_1987_Electronic_Arts_PAL.adf reference/adf
```

Le programme vérifie les sommes de contrôle des blocs OFS, la taille des fichiers et l'absence de cycles dans les chaînes. Toutes les entrées du disque ont été extraites. `adf/` contient aussi les commandes Workbench, périphériques et bibliothèques du disque pour permettre l'inspection ; ils ne sont pas nécessaires au moteur Go.

Source primaire complémentaire : [manuel Electronic Arts, DeluxePaint II](https://d1yx3ys82bpsa0.cloudfront.net/atchm/documents/DeluxePaint_II_manual.pdf), copie locale `DeluxePaint_II_manual.pdf`, 152 pages. La pagination imprimée diffère de celle du PDF. Figure 1.1, page imprimée 1.5, correspond à la page PDF 25 ; la section de référence commence page PDF 105. `manual-toolbox.png` présente la figure et sa légende. Les données extraites du binaire font autorité lorsqu'elles diffèrent du manuel.

## Interface confirmée

Le panneau se trouve à droite de la surface de peinture. En haut : dix brosses prédéfinies, quatre rondes, quatre carrées et deux pointillées. Puis deux colonnes d'outils, rangées de haut en bas :

| Gauche | Droite |
|---|---|
| Dotted Freehand | Continuous Freehand |
| Straight Line | Curve |
| Fill | Airbrush |
| Unfilled/filled Rectangle | Unfilled/filled Circle |
| Unfilled/filled Ellipse | Unfilled/filled Polygon |
| Brush Selector | Text |
| Grid | Symmetry |
| Magnify | Zoom |
| UNDO | CLR |

Sous ces outils : indicateur circulaire de couleur de premier plan sur la couleur de fond, puis palette 32 couleurs, quatre colonnes sur huit lignes. Le bouton gauche peint et sélectionne la couleur de premier plan ; le bouton droit peint et sélectionne la couleur de fond. Le clic droit sur l'indicateur ouvre la palette ; sur une brosse prédéfinie, il permet d'en régler la taille.

Les menus sont accessibles en maintenant le bouton droit en haut de l'écran ; ils s'ouvrent au passage de la souris, les sous-menus s'étendent à droite et la commande est choisie au relâchement. Au repos, la barre affiche le mode courant, éventuellement `S` pour le stencil, `B` pour le fond fixé, les coordonnées ou les angles de perspective. F9 masque la barre et F10 masque la barre et le panneau.

## Menus

Ordre confirmé : **Picture, Brush, Mode, Effects, Font, Prefs**. Les libellés suivants proviennent du binaire ou du manuel primaire ; les raccourcis sensibles à la casse sont conservés.

- **Picture** : Load, Save, Delete, Print ; Color Control → Palette (`p`), Use Brush Palette, Restore Palette, Default Palette, Cycle (`TAB`), Bg → Fg, Bg ↔ Fg, Remap ; Spare → Swap (`j`), Copy To Spare, Merge in front, Merge in back, Delete this Page ; Page Size, Show Page (`S`), Screen Format, Quit, About.
- **Brush** : Load, Save, Delete ; Size → Stretch (`Z`), Halve (`h`), Double (`H`), Double Horiz, Double Vert ; Flip → Horiz (`x`), Vert (`y`) ; Rotate → 90 Degrees (`z`), Any Angle, Shear ; Change Color → Bg → Fg, Bg ↔ Fg, Remap ; Bend → Horiz, Vert ; Handle → Center, Corner.
- **Mode** : Matte (`F1`), Color (`F2`), Replc (`F3`), Smear (`F4`), Shade (`F5`), Blend (`F6`), Cycle (`F7`), Smooth (`F8`).
- **Effects** : Stencil → Make, Remake, Lock FG, Reverse, On/Off, Free, Load, Save, Delete ; Background → Fix, Off ; Perspective → Do, FillScreen, Reset, Center, Anti-Alias (None, Low, High), Rotation (Absolute, Relative).
- **Font** : Style → Bold, Italic, Underln ; Load Font Dir, puis les familles et tailles chargées depuis le disque.
- **Prefs** : Coords, Fast FB, MultiCycle, Be Square, Workbench, ExclBrush.

Le fond fixé par **Background → Fix** est une copie de l'image courante : les ajouts se dessinent dessus, tandis que **CLR** et la peinture au bouton droit restaurent le fond dans les zones effacées. **Off** libère cette copie et rétablit l'effacement normal de toute la page. **Stencil → Lock FG** protège les zones peintes depuis Fix, indépendamment de leurs couleurs (manuel, pages 4.17–4.18). Il ne faut donc pas assimiler le fond fixé à une simple couleur uniforme.

`dpaint-strings.txt` fournit les offsets des libellés, requêtes et autres chaînes utiles dans le binaire, notamment la plage `0x1c03e–0x1c379`. Les premiers noms Picture/Brush et les opérations Load/Save/Delete sont réutilisés depuis des chaînes de requêtes plutôt que répétés dans ce bloc.

## Raccourcis des outils

| Touche | Action |
|---|---|
| `s`, `d`, `D` | Dessin pointillé, continu, continu avec brosse un pixel |
| `v`, `q`, `f`, `F` | Ligne, courbe, remplissage, réglages de remplissage |
| `r` / `R`, `c` / `C`, `e` / `E` | Rectangle, cercle, ellipse : contour / plein |
| `b`, `B`, `t` | Capture brosse, précédente brosse capturée, texte |
| `u`, `K` | Undo, clear |
| `m`, `<`, `>` | Loupe, réduction/augmentation du zoom |
| `g`, `G`, `/` | Grille, grille alignée au pinceau, symétrie |
| `,`, `.`, `[`, `]` | Pipette, brosse un pixel, couleur précédente/suivante de plage |
| `-`, `=` | Réduire/agrandir la brosse |
| `n`, flèches | Centrer sous le curseur, déplacer la page |
| `a`, Espace | Répéter la dernière commande de menu, annuler l'opération en cours |

Shift contraint lignes et formes ; Ctrl laisse des traces lors du tracé de ces outils. Le manuel associe F8 au curseur, alors que ce disque associe explicitement F8 à Smooth : Pixeluxe doit suivre la version disque pour ce point. La touche du polygone n'est pas établie par la table fournie.

## Images et palettes originales

Les quinze fichiers graphiques sont des IFF `FORM ILBM`, cinq plans, 32 couleurs ; compression brute ou ByteRun1. `asset-manifest.json` indique dimensions, palette, transparence, compression, coordonnées de poignée `GRAB`, cycles `CRNG` et SHA-256. `previews/` contient des PNG décodés pour comparaison visuelle. Les octets CMAP sont alignés sur quatre bits ; les aperçus rendent le niveau matériel 0–15 sur 0–255 par multiplication par 17.

| Fichier sous `adf/` | Dimensions |
|---|---|
| Lo-Res/Seascape | 320 × 200 |
| Lo-Res/StencilSet | 320 × 200 |
| Lo-Res/Reference Palette | 320 × 200 |
| Brush/Archbrush | 147 × 37 |
| Brush/Bobsled | 24 × 27 |
| Brush/Building | 53 × 106 |
| Brush/Dolphin | 103 × 128 |
| Brush/Pattern1 | 18 × 15 |
| Brush/fireworks | 107 × 104 |
| Brush/anim1, anim2, anim3 | 32 × 19 ; 83 × 65 ; 49 × 200 |
| Brush/anim4, anim5, anim6 | 319 × 54 ; 81 × 70 ; 93 × 45 |

Les répertoires Med-Res, Interlace et Hi-Res ne contiennent pas d'images sur ce disque. Les exemples Lo-Res ont été enregistrés avec un format 320 × 200 et un rapport de pixel 10:11 ; cela ne signifie pas que le mode PAL de l'application est limité à 200 lignes. Le binaire contient les hauteurs 200/400 et 256/512 pour NTSC/PAL, ainsi que le format 320 × 256 dans Building.

## Fontes et limites

`fonts.json` contient les glyphes bitmap originaux, leurs largeurs, avances, crénages et lignes de base. Les planches sont dans `previews/fonts/`. Familles : ruby 8/12/15, opal 9/12, sapphire 14/19, diamond 12/20, garnet 9/16, emerald 17/20 et topaz 11. Les codes de caractères sont Amiga/Latin-1, généralement `0x20–0xff`.

Le programme ouvre `topaz.font` pour l'interface. La fonte système ROM 8 pixels n'est pas présente sur l'ADF ; topaz 11 est la seule variante disque. Le disque seul ne permet donc pas de récupérer directement la fonte de tous les menus, ni de prouver leurs couleurs ou leur placement pixel par pixel sans exécuter le logiciel avec un environnement Amiga approprié. L'ordre précis de certains items est reconstruit à partir du manuel et des chaînes ; aucune capture d'exécution de cet ADF n'a été prétendue. Les images et fontes extraites servent de référence à une réimplémentation native Go, sans émulation du binaire 68000.
